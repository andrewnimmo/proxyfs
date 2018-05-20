package proxyfs

import (
	"context"
	"os"
	"regexp"
	"strings"

	"bazil.org/fuse"
)

// Creates a file from a pointer to a regexp.Regexp which is read and updated appropriately.
// Implements the FunctionReader and FunctionWriter interfaces
type RegexpFile struct {
	Data *regexp.Regexp
	Mode os.FileMode
}

var _ FunctionReader = (*RegexpFile)(nil)
var _ FunctionWriter = (*RegexpFile)(nil)

// NewRegexpFile returns a new RegexpFile using the given regexp.Regexp pointer
func NewRegexpFile(Data *regexp.Regexp) *RegexpFile {
	return &RegexpFile{Data: Data, Mode: 0666}
}

// Return the value of the regexp.Regexp
func (rf *RegexpFile) ReadAll(ctx context.Context) ([]byte, error) {
	if rf.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}
	return []byte((*rf.Data).String()), nil
}

// Modify the underlying regexp.Regexp
func (rf *RegexpFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if rf.Mode&0222 == 0 {
		return fuse.EPERM
	}

	c := strings.TrimSpace(string(req.Data))
	r, err := regexp.Compile(c)
	if err != nil {
		return fuse.ERANGE
	}

	*rf.Data = *r
	resp.Size = len(req.Data)
	return nil
}

// Implement Attr to implement the fs.Node interface
func (rf RegexpFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = rf.Mode
	attr.Size = uint64(len((*rf.Data).String()))
	return nil
}

// Implement Fsync to implement the fs.NodeFsyncer interface
func (RegexpFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
