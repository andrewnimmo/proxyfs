package main

import (
	"context"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type FunctionNode interface {
	fs.NodeFsyncer
}

type FunctionReader interface {
	fs.Node
	fs.HandleReadAller
	FunctionNode
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
