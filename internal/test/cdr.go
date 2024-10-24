package test

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"lesiw.io/cmdio"
)

type EchoCdr [][]string

var Uniq = make(map[string]int)

func (c *EchoCdr) Command(
	_ context.Context, env map[string]string, args ...string,
) cmdio.Command {
	*c = append(*c, args)
	s := fmt.Sprintf("%v", args)
	Uniq[s]++
	if Uniq[s] > 1 {
		s = fmt.Sprintf("%s[%d]", s, Uniq[s])
	}
	return rwcmd{bytes.NewBufferString(s + "\n")}
}

type rwcmd struct{ io.ReadWriter }

func (rwcmd) Close() error   { return nil }
func (rwcmd) String() string { return "<nop>" }
func (rwcmd) Attach() error  { return nil }
func (rwcmd) Code() int      { return 0 }
func (rwcmd) Log(io.Writer)  {}
