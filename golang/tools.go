package golang

import (
	"lesiw.io/cmdio"
)

func GolangCi() string {
	if which, err := Runner.Get("which", "golangci-lint"); err == nil {
		return which.Out
	}
	gopath := Runner.MustGet("go", "env", "GOPATH")
	cmdio.MustPipe(
		Runner.Command("curl", "-sSfL",
			"https://raw.githubusercontent.com/golangci"+
				"/golangci-lint/master/install.sh"),
		Runner.Command("sh", "-s", "--", "-b", gopath.Out+"/bin"),
	)
	return Runner.MustGet("which", "golangci-lint").Out
}

func GoTestSum() string {
	if which, err := Runner.Get("which", "gotestsum"); err == nil {
		return which.Out
	}
	Runner.MustRun("go", "install", "gotest.tools/gotestsum@latest")
	return Runner.MustGet("which", "gotestsum").Out
}
