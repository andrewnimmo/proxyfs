package proxyfs

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"bazil.org/fuse"
)

// Creates a file from a point64er to a int64 which is read and updated appropriately. Implements the FunctionNode int64erface.
type Int64File struct {
	File
	Data *int64
	Lock *sync.RWMutex
}

var _ FunctionNode = (*Int64File)(nil)

// NewInt64File returns a new Int64File using the given int64 point64er
func NewInt64File(Data *int64) *Int64File {
	ret := &Int64File{Data: Data, Lock: &sync.RWMutex{}}
	ret.Mode = 0666
	return ret
}

// Return the value of the int64
func (f *Int64File) ReadAll(ctx context.Context) ([]byte, error) {
	if f.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}

	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return []byte(strconv.FormatInt(*f.Data, 10)), nil
}

// Returns the length of the underlying int64
func (f *Int64File) Length(ctx context.Context) (int, error) {
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return len(strconv.FormatInt(*f.Data, 10)), nil
}

// Modify the underlying int64
func (f *Int64File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if f.Mode&0222 == 0 {
		return fuse.EPERM
	}

	i, err := strconv.ParseInt(strings.TrimSpace(string(req.Data)), 10, 64)
	if err != nil {
		return fuse.ERANGE
	}

	f.Lock.Lock()
	(*f.Data) = i
	resp.Size = len(req.Data)
	f.Lock.Unlock()
	return nil
}

// Implement Attr to implement the fs.Node int64erface
func (f Int64File) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = f.Mode
	f.Lock.RLock()
	defer f.Lock.RUnlock()
	attr.Size = uint64(len(strconv.FormatInt(*f.Data, 10)))
	return nil
}

// Implement Fsync to implement the fs.NodeFsyncer int64erface
func (Int64File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

var _ FunctionNodeable = (*Int64File)(nil)

func (f *Int64File) Node() FunctionNode {
	return f
}
