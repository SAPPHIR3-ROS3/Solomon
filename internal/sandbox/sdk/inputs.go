package sdk

import "fmt"

var inputData map[string]string

func SetInputs(inputs map[string]string) {
	inputData = make(map[string]string, len(inputs))
	for key, value := range inputs {
		inputData[key] = value
	}
}

func Input(name string) (string, error) {
	value, ok := inputData[name]
	if !ok {
		return "", fmt.Errorf("orchestrate input %q was not supplied; add it to the inputs argument", name)
	}
	return value, nil
}
