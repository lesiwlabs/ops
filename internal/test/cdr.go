package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

type EchoCdr [][]string

var Uniq = make(map[string]int)

func (c *EchoCdr) Command(
	_ context.Context, env map[string]string, args ...string,
) io.ReadWriter {
	*c = append(*c, args)
	s := fmt.Sprintf("%v", args)
	if n, ok := Uniq[s]; ok {
		s = fmt.Sprintf("[%d]", n+1)
	}
	Uniq[s]++
	return bytes.NewBufferString(s + "\n")
}
