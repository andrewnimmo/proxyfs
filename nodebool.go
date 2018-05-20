package proxyfs

import (
	"context"
	"os"
	"strings"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a bool which is read and updated appropriately. Implements
// the FunctionReader and FunctionWriter interfaces
type BoolFile struct {
	Data *bool
	Mode os.FileMode
}

var _ FunctionReader = (*BoolFile)(nil)
var _ FunctionWriter = (*BoolFile)(nil)

// NewBoolFile returns a new BoolFile using the given bool pointer
func NewBoolFile(Data *bool) *BoolFile {
	return &BoolFile{Data: Data, Mode: 0666}
}

// Return the value of the bool
func (bf *BoolFile) ReadAll(ctx context.Context) ([]byte, error) {
	if bf.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}

	if *bf.Data {
		return []byte("1"), nil
	} else {
		return []byte("0"), nil
	}
}

// Modify the underlying bool
func (bf *BoolFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if bf.Mode&0222 == 0 {
		return fuse.EPERM
	}

	c := strings.TrimSpace(string(req.Data))

	if c == "0" {
		*bf.Data = false
		resp.Size = len(req.Data)
		return nil
	}

	if c == "1" {
		*bf.Data = true
		resp.Size = len(req.Data)
		return nil
	}

	return fuse.ERANGE
}

// Implement Attr to implement the fs.Node interface
func (bf BoolFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = bf.Mode
	attr.Size = 1
	return nil
}

// Implement Fsync to implement the fs.NodeFsyncer interface
func (BoolFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
