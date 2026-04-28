package tooling

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

func FuncName(fn any) (string, error) {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f != nil {
		return f.Name(), nil
	}
	return "", fmt.Errorf("invalid function")
}

func FuncSignature(fn any) (string, error) {
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return "", fmt.Errorf("not a function")
	}

	f := runtime.FuncForPC(v.Pointer())
	if f == nil {
		return "", fmt.Errorf("cannot resolve function name")
	}

	t := v.Type()

	in := make([]string, 0, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		typ := t.In(i).String()
		if t.IsVariadic() && i == t.NumIn()-1 {
			typ = "..." + strings.TrimPrefix(typ, "[]")
		}
		in = append(in, fmt.Sprintf("p%d %s", i, typ))
	}

	out := make([]string, 0, t.NumOut())
	for i := 0; i < t.NumOut(); i++ {
		typ := t.Out(i).String()
		name := fmt.Sprintf("r%d", i)
		if typ == "error" {
			name = "err"
		}
		out = append(out, fmt.Sprintf("%s %s", name, typ))
	}

	sig := fmt.Sprintf("%s(%s)", f.Name(), strings.Join(in, ", "))

	switch len(out) {
	case 0:
		return sig, nil
	case 1:
		return sig + " " + out[0], nil
	default:
		return sig + " (" + strings.Join(out, ", ") + ")", nil
	}
}

func BaseNameFromFuncName(full string) string {
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:]
	}
	return full
}
