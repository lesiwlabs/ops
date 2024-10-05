package golang

import (
	"lesiw.io/cmdio"
)

func GolangCi(rnr *cmdio.Runner) string {
	if which, err := rnr.Get("which", "golangci-lint"); err == nil {
		return which.Out
	}
	gopath := rnr.MustGet("go", "env", "GOPATH")
	cmdio.MustPipe(
		rnr.Command("curl", "-sSfL",
			"https://raw.githubusercontent.com/golangci"+
				"/golangci-lint/master/install.sh"),
		rnr.Command("sh", "-s", "--", "-b", gopath.Out+"/bin"),
	)
	return rnr.MustGet("which", "golangci-lint").Out
}

func GoTestSum(rnr *cmdio.Runner) string {
	if which, err := rnr.Get("which", "gotestsum"); err == nil {
		return which.Out
	}
	rnr.MustRun("go", "install", "gotest.tools/gotestsum@latest")
	return rnr.MustGet("which", "gotestsum").Out
}
