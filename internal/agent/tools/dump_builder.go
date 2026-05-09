package tools

import "fmt"

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
