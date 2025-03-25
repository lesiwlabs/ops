package golang

import (
	"sync"

	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sub"
)

var GolangCi = sync.OnceValue(func() *cmdio.Runner {
	if which, err := Builder().Get("which", "golangci-lint"); err == nil {
		return Builder().WithCommander(sub.New(which.Out).Commander)
	}
	// https://github.com/golangci/golangci-lint/issues/966
	Builder().MustRun("go", "install",
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest")
	which := Builder().MustGet("which", "golangci-lint")
	return Builder().WithCommander(sub.New(which.Out).Commander)
})

var GoTestSum = sync.OnceValue(func() *cmdio.Runner {
	if which, err := Builder().Get("which", "gotestsum"); err == nil {
		return Builder().WithCommander(sub.New(which.Out).Commander)
	}
	Builder().MustRun("go", "install", "gotest.tools/gotestsum@latest")
	which := Builder().MustGet("which", "gotestsum")
	return Builder().WithCommander(sub.New(which.Out).Commander)
})
