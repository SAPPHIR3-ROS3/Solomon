package agentruntime

import (
	"io"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

type runtimeBootstrapOut struct {
	out io.Writer
}

func (r runtimeBootstrapOut) Print(msg string) {
	if r.out == nil {
		return
	}
	_, _ = io.WriteString(r.out, termcolor.SystemMessageText(msg)+"\n")
}

var _ cursorint.BootstrapIO = runtimeBootstrapOut{}
