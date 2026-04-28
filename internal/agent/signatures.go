package agent

func signatureCreatePlan(name string, planText string) {}

func signatureEditPlan(name, oldStr, newStr string) {}

func signatureBuildPlan(name string) {}

func signatureShell(command string) {}

func signatureReadFile(path string) {}

func signatureEditFile(path, oldString, newString string) {}

func signatureSubagent(sysPromptPath, task string) {}
