package proxyfs

import (
	"context"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a bool which is read and updated appropriately. Implements
// the FunctionNode interface
type BoolFile struct {
	File
	Data *bool
	Lock *sync.RWMutex
}

var _ FunctionNode = (*BoolFile)(nil)

// NewBoolFile returns a new BoolFile using the given bool pointer
func NewBoolFile(Data *bool) *BoolFile {
	ret := &BoolFile{Data: Data, Lock: &sync.RWMutex{}}
	ret.Mode = 0666
	return ret
}

// Return the value of the bool
func (bf *BoolFile) ReadAll(ctx context.Context) ([]byte, error) {
	if bf.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}

	bf.Lock.RLock()
	defer bf.Lock.RUnlock()
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

	bf.Lock.Lock()
	defer bf.Lock.Unlock()
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

var _ FunctionNodeable = (*BoolFile)(nil)

func (bf *BoolFile) Node() FunctionNode {
	return bf
}
