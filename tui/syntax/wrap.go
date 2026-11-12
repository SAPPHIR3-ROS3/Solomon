package syntax

import (
	"regexp"
	"strings"
)

var sgrPattern = regexp.MustCompile("\\x1b\\[[0-9;:]*m")

func RestoreWrappedStyles(lines []string) []string {
	result := make([]string, len(lines))
	active := ""
	for index, line := range lines {
		prefix := active
		for _, sequence := range sgrPattern.FindAllString(line, -1) {
			parameters := sequence[2 : len(sequence)-1]
			if parameters == "" || parameters == "0" {
				active = ""
				continue
			}
			if strings.HasPrefix(parameters, "0;") {
				active = ""
			}
			active += sequence
		}
		result[index] = prefix + line
		if active != "" {
			result[index] += "\x1b[0m"
		}
	}
	return result
}
