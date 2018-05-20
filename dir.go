package main

import (
	"context"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type FunctionNode interface {
	GetMode() uint64
}

type FunctionReader interface {
	fs.Node
	fs.HandleReadAller
	FunctionNode
	Length(cts context.Context) (int, error)
}

type FunctionWriter interface {
	fs.Node
	fs.HandleWriter
	FunctionNode
}

// Dir represetns a directory in the filesystem. It contains subnodes of type Dir,
// FunctionReader, or FunctionWriter
type Dir struct {
	SubNodes map[string]fs.Node
}

func NewDir() *Dir {
	return &Dir{SubNodes: make(map[string]fs.Node)}
}

func (d *Dir) AddNode(name string, node fs.Node) {
	d.SubNodes[name] = node
}

var _ fs.Node = (*Dir)(nil)

// Attr is implemented to comply with the fs.Node interface. It sets the attributes
// of the filesystem
func (d Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.ModeDir | 0444
	return nil
}

func (d Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	node, ok := d.SubNodes[name]
	if !ok {
		return nil, fuse.ENOENT
	}
	return node, nil
}

func (d Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var subdirs []fuse.Dirent

	for name, node := range d.SubNodes {

		nodetype := fuse.DT_Dir
		if _, ok := node.(FunctionNode); ok {
			nodetype = fuse.DT_File
		}

		subdirs = append(subdirs, fuse.Dirent{Name: name, Type: nodetype})
	}

	return subdirs, nil
}

// Creates a file from a pointer to a string which is read and updated appropriately. Implements
// the FunctionReader and FunctionWriter interfaces
type StringFuncRW struct {
	data *string
	mode os.FileMode
}

// NewStringFuncRW returns a new StringFuncRW using the given string pointer
func NewStringFuncRW(data *string) *StringFuncRW {
	return &StringFuncRW{data: data, mode: 0666}
}

// Return the value of the string
func (rw *StringFuncRW) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte(*rw.data), nil
}

// Returns the length of the underlying string
func (rw *StringFuncRW) Length(ctx context.Context) (int, error) {
	return len(*rw.data), nil
}

// Modify the underlying string
func (rw *StringFuncRW) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	(*rw.data) = string(req.Data)
	resp.Size = len(*rw.data)
	return nil
}

// Implement Attr to implement the fs.Node interface
func (rw StringFuncRW) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = rw.mode
	attr.Size = uint64(len(*rw.data))
	return nil
}

func (rw *StringFuncRW) GetMode() os.FileMode {
	return rw.mode
}
