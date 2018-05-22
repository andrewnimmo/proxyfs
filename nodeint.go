package proxyfs

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a int which is read and updated appropriately. Implements the FunctionNode interface.
type IntFile struct {
	File
	Data *int
	Lock *sync.RWMutex
}

var _ FunctionNode = (*IntFile)(nil)

// NewIntFile returns a new IntFile using the given int pointer
func NewIntFile(Data *int) *IntFile {
	ret := &IntFile{Data: Data, Lock: &sync.RWMutex{}}
	ret.Mode = 0666
	return ret
}

// Return the value of the int
func (f *IntFile) ReadAll(ctx context.Context) ([]byte, error) {
	if f.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return []byte(strconv.Itoa(*f.Data)), nil
}

// Returns the length of the underlying int
func (f *IntFile) Length(ctx context.Context) (int, error) {
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return len(strconv.Itoa(*f.Data)), nil
}

// Modify the underlying int
func (f *IntFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if f.Mode&0222 == 0 {
		return fuse.EPERM
	}

	i, err := strconv.Atoi(strings.TrimSpace(string(req.Data)))
	if err != nil {
		return fuse.ERANGE
	}

	f.Lock.Lock()
	(*f.Data) = i
	resp.Size = len(req.Data)
	f.Lock.Unlock()
	return nil
}

// Implement Attr to implement the fs.Node interface
func (f IntFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = f.Mode
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	attr.Size = uint64(len(strconv.Itoa(*f.Data)))
	return nil
}

// Implement Fsync to implement the fs.NodeFsyncer interface
func (IntFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

var _ FunctionNodeable = (*IntFile)(nil)

func (f *IntFile) Node() FunctionNode {
	return f
}