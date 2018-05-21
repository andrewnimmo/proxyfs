package proxyfs

import (
	"context"
	"os"

	"bazil.org/fuse"
)

type File struct {
	Mode os.FileMode
}

func (f File) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = f.Mode
	return nil
}

func (f File) DirentType() fuse.DirentType {
	return fuse.DT_File
}
