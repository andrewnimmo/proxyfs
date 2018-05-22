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
	Lock *sync.RWMutex
}

var _ FunctionNode = (*URLFile)(nil)

// NewURLFile returns a new URLFile using the given url.URL pourl.URLer
func NewURLFile(Data *url.URL) *URLFile {
	ret := &URLFile{Data: Data, Lock: &sync.RWMutex{}}
	ret.Mode = 0666
	return ret
}

// Return the value of the url.URL
func (f *URLFile) ReadAll(ctx context.Context) ([]byte, error) {
	if f.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}

	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return []byte(f.Data.String()), nil
}

// Returns the length of the underlying url.URL
func (f *URLFile) Length(ctx context.Context) (int, error) {
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return len(f.Data.String()), nil
}

// Modify the underlying url.URL
func (f *URLFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if f.Mode&0222 == 0 {
		return fuse.EPERM
	}

	u, err := url.Parse(strings.TrimSpace(string(req.Data)))
	if err != nil {
		return fuse.ERANGE
	}

	f.Lock.Lock()
	(*f.Data) = *u
	resp.Size = len(req.Data)
	f.Lock.Unlock()
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

// Implement Fsync to implement the fs.NodeFsyncer url.URLerface
func (URLFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

var _ FunctionNodeable = (*URLFile)(nil)

func (f *URLFile) Node() FunctionNode {
	return f
}
