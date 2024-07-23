package clerkfs

import (
	"io/fs"

	"lesiw.io/ci"
	"lesiw.io/clerk"
)

var cfs = new(clerk.ClerkFS)

func init() {
	ci.PostHandle(func() {
		if err := cfs.Apply("."); err != nil {
			panic(err)
		}
	})
}

func Add(fsys fs.FS) error {
	return cfs.Add(fsys)
}
