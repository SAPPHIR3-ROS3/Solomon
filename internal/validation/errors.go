package validation

import (
	"errors"
	"fmt"
)

type Issue struct {
	Kind      string
	Component string
	Field     string
	Problem   string
	Cause     error
}

func (e *Issue) Error() string {
	return fmt.Sprintf("%s %s error (%s): %s", e.Component, e.Kind, e.Field, e.Problem)
}
func (e *Issue) Unwrap() error { return e.Cause }

func Argument(component, field, problem string) error {
	return &Issue{Kind: "arguments", Component: component, Field: field, Problem: problem}
}

func Configuration(component, field, problem string) error {
	return &Issue{Kind: "configuration", Component: component, Field: field, Problem: problem}
}

func Kind(err error) string {
	var issue *Issue
	if errors.As(err, &issue) {
		return issue.Kind
	}
	return ""
}
