package clerkfs

import (
	"io/fs"
	"sync"

	"lesiw.io/clerk"
	"lesiw.io/ops"
)

var cfs = new(clerk.ClerkFS)
var once sync.Once

func Add(fsys fs.FS) error {
	return cfs.Add(fsys)
}

func Apply() {
	once.Do(func() {
		ops.PostHandle(func() {
			if err := cfs.Apply("."); err != nil {
				panic(err)
			}
		})
	})
}
