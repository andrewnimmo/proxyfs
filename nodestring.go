package proxyfs

import (
	"context"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a string which is read and updated appropriately. Implements the FunctionNode interface.
type StringFile struct {
	File
	Data *string
	Lock *sync.RWMutex
}

var _ FunctionNode = (*StringFile)(nil)

// NewStringFile returns a new StringFile using the given string pointer
func NewStringFile(Data *string) *StringFile {
	ret := &StringFile{Data: Data, Lock: &sync.RWMutex{}}
	ret.Mode = 0666
	return ret
}

// Return the value of the string
func (sf *StringFile) ReadAll(ctx context.Context) ([]byte, error) {
	if sf.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}
	sf.Lock.RLock()
	defer sf.Lock.RUnlock()
	return []byte(*sf.Data), nil
}

// Returns the length of the underlying string
func (sf *StringFile) Length(ctx context.Context) (int, error) {
	sf.Lock.RLock()
	defer sf.Lock.RUnlock()
	return len(*sf.Data), nil
}

// Modify the underlying string
func (sf *StringFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if sf.Mode&0222 == 0 {
		return fuse.EPERM
	}
	sf.Lock.Lock()
	defer sf.Lock.Unlock()
	(*sf.Data) = strings.TrimSpace(string(req.Data))
	resp.Size = len(req.Data)
	return nil
}

// Implement Attr to implement the fs.Node interface
func (sf StringFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = sf.Mode
	sf.Lock.RLock()
	defer sf.Lock.RUnlock()
	attr.Size = uint64(len(*sf.Data))
	return nil
}

// Implement Fsync to implement the fs.NodeFsyncer interface
func (StringFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

var _ FunctionNodeable = (*StringFile)(nil)

func (sf *StringFile) Node() FunctionNode {
	return sf
}
