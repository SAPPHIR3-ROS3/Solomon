//go:build windows

package server

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsTerminalProcess struct {
	input  *os.File
	output *os.File

	mu      sync.Mutex
	console windows.Handle
	process windows.Handle

	closeOnce sync.Once
	closeErr  error
	done      chan struct{}
	waitErr   error
}

func startTerminalProcess(options terminalProcessOptions) (_ terminalProcess, err error) {
	var inputRead, inputWrite windows.Handle
	var outputRead, outputWrite windows.Handle
	var console windows.Handle
	created := false
	defer func() {
		if created {
			return
		}
		if console != 0 {
			windows.ClosePseudoConsole(console)
		}
		closeWindowsHandle(inputRead)
		closeWindowsHandle(inputWrite)
		closeWindowsHandle(outputRead)
		closeWindowsHandle(outputWrite)
	}()

	if err := windows.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create terminal input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create terminal output pipe: %w", err)
	}
	if err := windows.CreatePseudoConsole(
		windows.Coord{X: int16(options.Cols), Y: int16(options.Rows)},
		inputRead,
		outputWrite,
		0,
		&console,
	); err != nil {
		return nil, fmt.Errorf("create Windows pseudoconsole: %w", err)
	}

	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, fmt.Errorf("create terminal process attributes: %w", err)
	}
	defer attributes.Delete()
	if err := attributes.Update(
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		unsafe.Pointer(console),
		unsafe.Sizeof(console),
	); err != nil {
		return nil, fmt.Errorf("set terminal pseudoconsole attribute: %w", err)
	}

	arguments := append([]string{options.Shell}, options.Args...)
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(arguments))
	if err != nil {
		return nil, fmt.Errorf("encode terminal command line: %w", err)
	}
	currentDirectory, err := windows.UTF16PtrFromString(options.Cwd)
	if err != nil {
		return nil, fmt.Errorf("encode terminal working directory: %w", err)
	}
	startup := &windows.StartupInfoEx{
		StartupInfo:             windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{}))},
		ProcThreadAttributeList: attributes.List(),
	}
	startup.StartupInfo.Flags |= windows.STARTF_USESTDHANDLES
	var processInfo windows.ProcessInformation
	if err := windows.CreateProcess(
		nil,
		commandLine,
		nil,
		nil,
		false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT,
		nil,
		currentDirectory,
		&startup.StartupInfo,
		&processInfo,
	); err != nil {
		return nil, fmt.Errorf("start terminal shell %q: %w", options.Shell, err)
	}
	closeWindowsHandle(processInfo.Thread)
	closeWindowsHandle(inputRead)
	inputRead = 0
	closeWindowsHandle(outputWrite)
	outputWrite = 0

	input := os.NewFile(uintptr(inputWrite), "terminal-input")
	output := os.NewFile(uintptr(outputRead), "terminal-output")
	if input == nil || output == nil {
		if input != nil {
			_ = input.Close()
			inputWrite = 0
		}
		if output != nil {
			_ = output.Close()
			outputRead = 0
		}
		_ = windows.TerminateProcess(processInfo.Process, 1)
		_, _ = windows.WaitForSingleObject(processInfo.Process, windows.INFINITE)
		closeWindowsHandle(processInfo.Process)
		return nil, errors.New("wrap terminal pseudoconsole pipes")
	}

	process := &windowsTerminalProcess{
		console: console,
		input:   input,
		output:  output,
		process: processInfo.Process,
		done:    make(chan struct{}),
	}
	console = 0
	inputWrite = 0
	outputRead = 0
	created = true
	go process.waitForExit()
	return process, nil
}

func (p *windowsTerminalProcess) Read(data []byte) (int, error) {
	return p.output.Read(data)
}

func (p *windowsTerminalProcess) Write(data []byte) (int, error) {
	return p.input.Write(data)
}

func (p *windowsTerminalProcess) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		console := p.console
		p.console = 0
		if console != 0 {
			windows.ClosePseudoConsole(console)
		}
		p.mu.Unlock()

		if err := p.input.Close(); err != nil {
			p.closeErr = err
		}
		if err := p.output.Close(); err != nil && p.closeErr == nil {
			p.closeErr = err
		}
	})
	return p.closeErr
}

func (p *windowsTerminalProcess) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.process == 0 {
		return nil
	}
	return windows.TerminateProcess(p.process, 1)
}

func (p *windowsTerminalProcess) Resize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.console == 0 {
		return os.ErrClosed
	}
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (p *windowsTerminalProcess) Wait() error {
	<-p.done
	return p.waitErr
}

func (p *windowsTerminalProcess) waitForExit() {
	p.mu.Lock()
	process := p.process
	p.mu.Unlock()
	if process == 0 {
		p.waitErr = errors.New("terminal process handle is closed")
		close(p.done)
		return
	}

	waitResult, err := windows.WaitForSingleObject(process, windows.INFINITE)
	if err != nil {
		p.waitErr = err
	} else if waitResult != windows.WAIT_OBJECT_0 {
		p.waitErr = fmt.Errorf("wait for terminal process: unexpected result 0x%x", waitResult)
	} else {
		var exitCode uint32
		if err := windows.GetExitCodeProcess(process, &exitCode); err != nil {
			p.waitErr = err
		} else if exitCode != 0 {
			p.waitErr = fmt.Errorf("terminal process exited with code %d", exitCode)
		}
	}
	_ = p.Close()
	p.mu.Lock()
	if p.process == process {
		closeWindowsHandle(p.process)
		p.process = 0
	}
	p.mu.Unlock()
	close(p.done)
}

func closeWindowsHandle(handle windows.Handle) {
	if handle != 0 {
		_ = windows.CloseHandle(handle)
	}
}
