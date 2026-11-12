package compile

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Failure struct {
	Kind        string
	Phase       string
	Retryable   bool
	RetryAction string
	Err         error
}

func (f *Failure) Error() string { return f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

var sourceLocation = regexp.MustCompile(`(?m)(?:^|\n)(?:\./)?main\.go:\d+(?::\d+)?:`)

func DescribeFailure(err error) *Failure {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure
	}
	return classifyFailure(err, "initialization")
}

func classifyFailure(err error, phase string) *Failure {
	f := &Failure{Kind: "environment", Phase: phase, RetryAction: "fix_environment", Err: err}
	text := strings.ToLower(err.Error())
	var pathError *os.PathError
	var execError *exec.Error
	var networkError net.Error
	switch {
	case errors.Is(err, context.Canceled):
		f.Kind, f.RetryAction = "cancelled", "none"
	case errors.Is(err, context.DeadlineExceeded):
		f.Kind, f.Retryable, f.RetryAction = "timeout", true, "retry_build"
	case errors.As(err, &networkError) && networkError.Timeout():
		f.Kind, f.Retryable, f.RetryAction = "timeout", true, "retry_build"
	case errors.As(err, &pathError):
		f.Kind = "filesystem"
	case errors.As(err, &execError):
		f.Kind = "toolchain"
	case phase == "parse":
		f.Kind, f.Retryable, f.RetryAction = "source", true, "fix_source"
	case containsFailureText(text, "permission denied", "no space left", "read-only file system", "disk full"):
		f.Kind = "filesystem"
	case containsFailureText(text, "i/o timeout", "context deadline exceeded", "client.timeout exceeded", "tls handshake timeout"):
		f.Kind, f.Retryable, f.RetryAction = "timeout", true, "retry_build"
	case containsFailureText(text, "dial tcp", "no such host", "connection reset", "tls handshake", "unexpected eof", "network is unreachable"):
		f.Kind, f.Retryable, f.RetryAction = "network", true, "retry_build"
	case sourceLocation.MatchString(err.Error()) || strings.Contains(text, "function main is undeclared"):
		f.Kind, f.Retryable, f.RetryAction = "source", true, "fix_source"
	case phase == "toolchain" || containsFailureText(text, "toolchain", "go executable", "go: cannot find goroot"):
		f.Kind = "toolchain"
	case phase == "module" || containsFailureText(text, "go.sum", "go.mod", "module lookup", "checksum mismatch"):
		f.Kind = "dependency"
	case phase == "storage" || phase == "artifact":
		f.Kind = "filesystem"
	}
	return f
}

func containsFailureText(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
