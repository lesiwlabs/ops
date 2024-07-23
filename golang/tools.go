package golang

import (
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
)

var Box *cmdio.Box = sys.Box()

func GolangCi() string {
	if which, err := Box.Get("which", "golangci-lint"); err == nil {
		return which.Output
	}
	gopath := Box.MustGet("go", "env", "GOPATH")
	cmdio.MustPipe(
		Box.Command("curl", "-sSfL",
			"https://raw.githubusercontent.com/golangci"+
				"/golangci-lint/master/install.sh"),
		Box.Command("sh", "-s", "--", "-b", gopath.Output+"/bin"),
	)
	return Box.MustGet("which", "golangci-lint").Output
}

func GoTestSum() string {
	if which, err := Box.Get("which", "gotestsum"); err == nil {
		return which.Output
	}
	Box.MustRun("go", "install", "gotest.tools/gotestsum@latest")
	return Box.MustGet("which", "gotestsum").Output
}
