package cursor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func Login(ctx context.Context, out io.Writer) (ts TokenSet, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "Cursor login failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	params, err := GenerateAuthParams()
	if err != nil {
		return TokenSet{}, err
	}
	if out != nil {
		fmt.Fprintln(out, "Opening browser for Cursor sign-in…")
		fmt.Fprintln(out, "Complete sign-in in the browser window that opens now.")
		fmt.Fprintf(out, "If the browser does not open, paste this URL into a new tab:\n%s\n", params.LoginURL)
	}
	_ = openBrowser(params.LoginURL)
	tokens, err := PollForAuth(ctx, params.UUID, params.Verifier)
	if err != nil {
		return TokenSet{}, err
	}
	if out != nil {
		fmt.Fprintln(out, "Cursor sign-in complete.")
	}
	return tokens, nil
}

func openBrowser(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("open browser: empty URL")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
