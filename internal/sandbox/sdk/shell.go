package sdk

import "fmt"

func Shell(command, intent string) (string, error) {
	r, err := shellCall(command, intent, 0)
	if err != nil {
		return "", err
	}
	if r.Exit != 0 {
		return r.Output, fmt.Errorf("shell exit %d: %s", r.Exit, r.Output)
	}
	return r.Output, nil
}

func ShellWithTimeout(command, intent string, secs int) (string, error) {
	r, err := shellCall(command, intent, secs)
	if err != nil {
		return "", err
	}
	if r.Exit != 0 {
		return r.Output, fmt.Errorf("shell exit %d: %s", r.Exit, r.Output)
	}
	return r.Output, nil
}

func ShellResult(command, intent string) (ShellOutput, error) {
	return shellCall(command, intent, 0)
}

func ShellResultWithTimeout(command, intent string, secs int) (ShellOutput, error) {
	return shellCall(command, intent, secs)
}

func shellCall(command, intent string, secs int) (ShellOutput, error) {
	args := map[string]any{"command": command, "intent": intent}
	if secs > 0 {
		args["timeoutSeconds"] = secs
	}
	raw, err := callTool("shell", args)
	if err != nil {
		return ShellOutput{}, err
	}
	m, err := unmarshalToolMap(raw)
	if err != nil {
		return ShellOutput{}, err
	}
	return parseShellResult(m), nil
}
