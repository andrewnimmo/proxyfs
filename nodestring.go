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
}

var _ FunctionNode = (*StringFile)(nil)

// NewStringFile returns a new StringFile using the given string pointer
func NewStringFile(Data *string) *StringFile {
	ret := &StringFile{Data: Data}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

// Return the value of the string
func (sf *StringFile) valRead(ctx context.Context) ([]byte, error) {
	return []byte(*sf.Data), nil
}

// Modify the underlying string
func (sf *StringFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
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

var _ FunctionNodeable = (*StringFile)(nil)

func (sf *StringFile) Node() FunctionNode {
	return sf
}
