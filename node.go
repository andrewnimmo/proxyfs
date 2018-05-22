package proxyfs

import (
	"context"
	"os"
	"sync"

	"bazil.org/fuse"
)

type File struct {
	// The file mode.
	Mode os.FileMode

	// A channel that is written to when the value is updated to notify of
	// a change.
	Change chan int

	// A lock used to synchronise reads/writes
	Lock *sync.RWMutex

	// ValRead should return the value of the underlying data converted to
	// []byte, and any errors. ctx is passed in from ReadAll, and the return
	// value is used as the return value of ReadAll.
	//
	// This function is intended to be masked by any struct that embeds File.
	ValRead func(ctx context.Context) ([]byte, error)

	// ValWrite should modify the underlying data from the data given in req, as
	// well as setting resp.Size to reflect how much data was written. The
	// arguments are passed in from Write, and the return value is used as the
	// return value of Write.
	//
	// This function is intended to be masked by any structs that embed File.
	ValWrite func(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error
}

func (f File) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = f.Mode
	return nil
}

func (f File) DirentType() fuse.DirentType {
	return fuse.DT_File
}

// Implement the fs.HandleReadAller interface, with a call to ValRead which
// should be masked by structs which embed File. This function also makes a
// RLock and RUnlock calls to the Lock, as well as checking the permissions
// from the value of Mode.
func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	if f.Mode&0444 == 0 {
		return nil, fuse.EPERM
	}

	f.Lock.RLock()
	defer f.Lock.RUnlock()
	return f.ValRead(ctx)
}

// Implement the fs.HandleWriter interface, with a call to ValWrite which
// should be masked by structs that embed File. If the Change channel is not
// empty, a value is  sent through it to signal a change in the data to any
// listening routines. This function also makes Lock and Unlock calls to the
// Lock, as well as checking permissions from the value of Mode.
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if f.Mode&0222 == 0 {
		return fuse.EPERM
	}

	defer func() {
		select {
		case f.Change <- 1:
		default:
		}
	}()

	f.Lock.Lock()
	defer f.Lock.Unlock()
	return f.ValWrite(ctx, req, resp)
}

// Implement Fsync to implement the fs.NodeFsyncer interface
func (BoolFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
