package git

import (
	"sync"

	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sub"
)

var Runner = sync.OnceValue(func() *cmdio.Runner {
	return sub.New("git")
})
