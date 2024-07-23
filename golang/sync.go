package golang

import (
	"embed"

	"labs.lesiw.io/ci/clerkfs"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
)

//go:embed .*
var f embed.FS
var Box *cmdio.Box = sys.Env(map[string]string{})

type Actions struct{}

func (a Actions) Sync() {
	if err := clerkfs.Add(f); err != nil {
		panic(err)
	}
	clerkfs.Apply()
}

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
