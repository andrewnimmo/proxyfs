package proxyfs

import (
	"context"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a bool which is read and updated appropriately.
// New values are sent down the included channel. Implements the FunctionNode interface.
type BoolFile struct {
	File
	Data *bool
}

var _ FunctionNode = (*BoolFile)(nil)

// NewBoolFile returns a new BoolFile using the given bool pointer
func NewBoolFile(Data *bool) *BoolFile {
	ret := &BoolFile{Data: Data}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	ret.Change = make(chan int)
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

// Return the value of the bool
func (bf *BoolFile) valRead(ctx context.Context) ([]byte, error) {
	if *bf.Data {
		return []byte("1"), nil
	} else {
		return []byte("0"), nil
	}
}

// Modify the underlying bool
func (bf *BoolFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	c := strings.TrimSpace(string(req.Data))
	if c == "0" {
		*bf.Data = false
	} else if c == "1" {
		*bf.Data = true
	} else {
		return fuse.ERANGE
	}

	resp.Size = len(req.Data)
	return nil
}

// Implement Attr to implement the fs.Node interface
func (bf BoolFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = bf.Mode
	attr.Size = 1
	return nil
}

var _ FunctionNodeable = (*BoolFile)(nil)

func (bf *BoolFile) Node() FunctionNode {
	return bf
}
