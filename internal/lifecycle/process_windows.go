package lifecycle

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

func processIdentity(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != 259 {
		return ""
	}
	var creation, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &creation, &exit, &kernel, &user) != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", creation.HighDateTime, creation.LowDateTime)
}
func processExecutable(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	data := make([]uint16, 32768)
	size := uint32(len(data))
	if windows.QueryFullProcessImageName(h, 0, &data[0], &size) != nil {
		return ""
	}
	return windows.UTF16ToString(data[:size])
}
func installedDesktopProcesses(executable string) ([]desktopUpdateProcess, error) {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var result []desktopUpdateProcess
	for err := windows.Process32First(h, &entry); err == nil; err = windows.Process32Next(h, &entry) {
		pid := int(entry.ProcessID)
		if pid <= 1 || !sameExecutable(processExecutable(pid), executable) {
			continue
		}
		identity := processIdentity(pid)
		if identity != "" {
			result = append(result, desktopUpdateProcess{pid: pid, identity: identity, executable: executable, env: os.Environ()})
		}
	}
	return result, nil
}
func configureClientProcess(command *exec.Cmd, kind string) {
	flags := uint32(windows.CREATE_NEW_PROCESS_GROUP)
	if kind == "tui" {
		flags |= windows.CREATE_NEW_CONSOLE
	} else {
		flags |= windows.DETACHED_PROCESS
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
}

// Old builds have no registration handshake. WM_CLOSE asks their window to quit.
func stopLegacyDesktop(pid int) error {
	user := windows.NewLazySystemDLL("user32.dll")
	getPID := user.NewProc("GetWindowThreadProcessId")
	post := user.NewProc("PostMessageW")
	callback := syscall.NewCallback(func(hwnd uintptr, param uintptr) uintptr {
		var windowPID uint32
		getPID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if int(windowPID) == pid {
			post.Call(hwnd, 0x0010, 0, 0)
		}
		return 1
	})
	result, _, err := user.NewProc("EnumWindows").Call(callback, 0)
	if result == 0 {
		return err
	}
	return nil
}

// Go exec wires nil standard streams to NUL. A newly opened TUI console needs
// Windows to create its own standard handles instead.
func startClientProcess(command *exec.Cmd, kind string) error {
	if kind != "tui" {
		return command.Start()
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(command.Args))
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(command.Dir)
	if err != nil {
		return err
	}
	variables := map[string]string{}
	for _, entry := range command.Env {
		if len(entry) < 2 {
			continue
		}
		i := strings.IndexByte(entry[1:], '=')
		if i < 0 {
			continue
		}
		i++
		variables[strings.ToUpper(entry[:i])] = entry
	}
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var block []uint16
	for _, key := range keys {
		block = append(block, utf16.Encode([]rune(variables[key]))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	if len(block) == 1 {
		block = append(block, 0)
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var process windows.ProcessInformation
	if err := windows.CreateProcess(nil, line, nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NEW_CONSOLE|windows.CREATE_NEW_PROCESS_GROUP, &block[0], cwd, &startup, &process); err != nil {
		return err
	}
	windows.CloseHandle(process.Thread)
	windows.CloseHandle(process.Process)
	command.Process, err = os.FindProcess(int(process.ProcessId))
	return err
}
