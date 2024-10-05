package golang

import (
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
)

var Rnr *cmdio.Runner = sys.Runner()

func GolangCi() string {
	if which, err := Rnr.Get("which", "golangci-lint"); err == nil {
		return which.Out
	}
	gopath := Rnr.MustGet("go", "env", "GOPATH")
	cmdio.MustPipe(
		Rnr.Command("curl", "-sSfL",
			"https://raw.githubusercontent.com/golangci"+
				"/golangci-lint/master/install.sh"),
		Rnr.Command("sh", "-s", "--", "-b", gopath.Out+"/bin"),
	)
	return Rnr.MustGet("which", "golangci-lint").Out
}

func GoTestSum() string {
	if which, err := Rnr.Get("which", "gotestsum"); err == nil {
		return which.Out
	}
	Rnr.MustRun("go", "install", "gotest.tools/gotestsum@latest")
	return Rnr.MustGet("which", "gotestsum").Out
}
