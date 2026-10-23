package server

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const viteSupervisorArg = "--solomon-vite-supervisor"

type viteLifetime struct {
	pid   int
	input io.Closer
	done  chan struct{}
}

var viteLifetimes sync.Map // *exec.Cmd -> *viteLifetime

// The private helper also works when the runtime is embedded in a test binary.
func init() {
	if len(os.Args) < 3 || os.Args[1] != viteSupervisorArg {
		return
	}
	if err := superviseVite(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func superviseVite(args []string) error {
	release, err := containViteTree()
	if err != nil {
		return err
	}
	defer release()
	command := exec.Command(args[0], args[1:]...)
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	configureManagedProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, command.Process.Pid); err != nil {
		killManagedTree(command.Process.Pid)
		_ = command.Wait()
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	ownerClosed := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); close(ownerClosed) }()
	select {
	case err := <-exited:
		// The leader may have crashed while its children are still running.
		killManagedTree(command.Process.Pid)
		return err
	case <-ownerClosed:
		killManagedTree(command.Process.Pid)
		<-exited
		return nil
	}
}

func startSupervisedVite(command *exec.Cmd) (*exec.Cmd, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	helper := exec.Command(executable, append([]string{viteSupervisorArg}, command.Args...)...)
	helper.Dir, helper.Env = command.Dir, command.Env
	helper.Stderr = command.Stderr
	output, err := helper.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer output.Close()
	input, err := helper.StdinPipe()
	if err != nil {
		return nil, err
	}
	configureManagedProcess(helper)
	if err := helper.Start(); err != nil {
		_ = input.Close()
		return nil, err
	}
	lifetime := &viteLifetime{input: input, done: make(chan struct{})}
	viteLifetimes.Store(helper, lifetime)
	go func() { _ = helper.Wait(); close(lifetime.done) }()
	ready := make(chan int, 1)
	go func() {
		line, _ := bufio.NewReader(output).ReadString('\n')
		pid, _ := strconv.Atoi(strings.TrimSpace(line))
		ready <- pid
	}()
	select {
	case lifetime.pid = <-ready:
	case <-time.After(5 * time.Second):
	}
	_ = output.Close()
	if lifetime.pid <= 1 {
		stopSupervisedVite(helper)
		return nil, fmt.Errorf("Vite supervisor did not start a child")
	}
	return helper, nil
}

func stopSupervisedVite(command *exec.Cmd) bool {
	value, ok := viteLifetimes.LoadAndDelete(command)
	if !ok {
		return false
	}
	lifetime := value.(*viteLifetime)
	_ = lifetime.input.Close()
	<-lifetime.done
	return true
}

func supervisedViteExited(command *exec.Cmd) bool {
	if value, ok := viteLifetimes.Load(command); ok {
		select {
		case <-value.(*viteLifetime).done:
			return true
		default:
		}
	}
	return false
}

func managedVitePID(command *exec.Cmd) int {
	if value, ok := viteLifetimes.Load(command); ok {
		return value.(*viteLifetime).pid
	}
	return command.Process.Pid
}
