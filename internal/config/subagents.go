package config

const DefaultSubagentMaxDepth = 4

func SubagentDepthLimit(root *Root) int {
	if root == nil || root.SubagentMaxDepth <= 0 {
		return DefaultSubagentMaxDepth
	}
	return root.SubagentMaxDepth
}
