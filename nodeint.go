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
}

var _ FunctionNode = (*IntFile)(nil)

// NewIntFile returns a new IntFile using the given int pointer
func NewIntFile(Data *int) *IntFile {
	ret := &IntFile{Data: Data}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	return ret
}

// Return the value of the int
func (f *IntFile) valRead(ctx context.Context) ([]byte, error) {
	return []byte(strconv.Itoa(*f.Data)), nil
}

// Modify the underlying int
func (f *IntFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	i, err := strconv.Atoi(strings.TrimSpace(string(req.Data)))
	if err != nil {
		return fuse.ERANGE
	}

	(*f.Data) = i
	resp.Size = len(req.Data)
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

var _ FunctionNodeable = (*IntFile)(nil)

func (f *IntFile) Node() FunctionNode {
	return f
}
