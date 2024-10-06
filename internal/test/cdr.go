package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

type EchoCdr [][]string

func (c *EchoCdr) Command(
	_ context.Context, env map[string]string, args ...string,
) io.ReadWriter {
	*c = append(*c, args)
	return bytes.NewBufferString(fmt.Sprintf("%v\n", args))
}
