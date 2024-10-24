package golang

func GolangCi() string {
	if which, err := Busybox().Get("which", "golangci-lint"); err == nil {
		return which.Out
	}
	// https://github.com/golangci/golangci-lint/issues/966
	Runner().MustRun("go", "install",
		"github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	return Busybox().MustGet("which", "golangci-lint").Out
}

func GoTestSum() string {
	if which, err := Busybox().Get("which", "gotestsum"); err == nil {
		return which.Out
	}
	Runner().MustRun("go", "install", "gotest.tools/gotestsum@latest")
	return Busybox().MustGet("which", "gotestsum").Out
}
