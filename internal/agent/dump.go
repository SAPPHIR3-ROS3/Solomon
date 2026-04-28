package agent

import (
	"fmt"

	"solomon/internal/tooling"
)

func buildPlanToolDump() (string, error) {
	b := &dumpBuilder{}
	sig, err := tooling.FuncSignature(signatureCreatePlan)
	if err != nil {
		return "", err
	}
	b.addBlock("createPlan", "Create or overwrite a plan file (markdown) under the project plans directory.", sig)
	sig, err = tooling.FuncSignature(signatureEditPlan)
	if err != nil {
		return "", err
	}
	b.addBlock("editPlan", "Replace first occurrence of old segment in plan file.", sig)
	sig, err = tooling.FuncSignature(signatureBuildPlan)
	if err != nil {
		return "", err
	}
	b.addBlock("buildPlan", "Switch to BUILD mode and run an implementation session for the named plan.", sig)
	return b.String(), nil
}

func buildBuildToolDump() (string, error) {
	b := &dumpBuilder{}
	sig, err := tooling.FuncSignature(signatureShell)
	if err != nil {
		return "", err
	}
	b.addBlock("shell", "Run a shell command in the harness working directory. Optional JSON fields may tweak behavior.", sig)
	sig, err = tooling.FuncSignature(signatureReadFile)
	if err != nil {
		return "", err
	}
	b.addBlock("readFile", "Read a text file relative to project root.", sig)
	sig, err = tooling.FuncSignature(signatureEditFile)
	if err != nil {
		return "", err
	}
	b.addBlock("editFile", "Replace oldString once with newString, or write newString when oldString empty.", sig)
	sig, err = tooling.FuncSignature(signatureSubagent)
	if err != nil {
		return "", err
	}
	b.addBlock("subagent", "Run a nested agent with system prompt from file and task string.", sig)
	return b.String(), nil
}

type dumpBuilder struct {
	s string
}

func (b *dumpBuilder) addBlock(name, desc, sig string) {
	if b.s != "" {
		b.s += "\n---\n"
	}
	b.s += fmt.Sprintf("name: %s\ndescription: %s\nsignature: %s\n", name, desc, sig)
}

func (b *dumpBuilder) String() string { return b.s }
