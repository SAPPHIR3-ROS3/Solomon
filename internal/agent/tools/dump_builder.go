package tools

import (
	"fmt"
	"strings"
)

type dumpBuilder struct {
	s string
}

func (b *dumpBuilder) addBlock(name, desc, sig string) {
	if b.s != "" {
		b.s += "\n---\n"
	}
	b.s += fmt.Sprintf("name: %s\ndescription: %s\nsignature: %s\n", name, desc, signatureWithRequiredIntent(sig))
}

func (b *dumpBuilder) String() string { return b.s }

func signatureWithRequiredIntent(sig string) string {
	open := strings.IndexByte(sig, '(')
	if open < 0 {
		return sig
	}
	close := strings.IndexByte(sig[open+1:], ')')
	if close < 0 {
		return sig
	}
	close += open + 1
	params := strings.TrimSpace(sig[open+1 : close])
	if strings.Contains(params, "intent string") {
		return sig
	}
	if params == "" {
		params = "intent string"
	} else {
		params += ", intent string"
	}
	return sig[:open+1] + params + sig[close:]
}
