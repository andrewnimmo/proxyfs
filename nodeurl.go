package proxyfs

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a url.URL which is read and updated appropriately. Implements the FunctionNode interface.
type URLFile struct {
	File
	Data *url.URL
}

var _ FunctionNode = (*URLFile)(nil)

// NewURLFile returns a new URLFile using the given url.URL pourl.URLer
func NewURLFile(Data *url.URL) *URLFile {
	ret := &URLFile{Data: Data}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

// Return the value of the url.URL
func (f *URLFile) valRead(ctx context.Context) ([]byte, error) {
	return []byte(f.Data.String()), nil
}

// Modify the underlying url.URL
func (f *URLFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	u, err := url.Parse(strings.TrimSpace(string(req.Data)))
	if err != nil {
		return fuse.ERANGE
	}

	(*f.Data) = *u
	resp.Size = len(req.Data)
	return nil
}

// Implement Attr to implement the fs.Node url.URLerface
func (f URLFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = f.Mode
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	attr.Size = uint64(len(f.Data.String()))
	return nil
}

var _ FunctionNodeable = (*URLFile)(nil)

func (f *URLFile) Node() FunctionNode {
	return f
}
