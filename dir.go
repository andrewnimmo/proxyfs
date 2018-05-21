package proxyfs

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type FunctionNode interface {
	fs.Node
	fs.HandleReadAller
	fs.HandleWriter
}

// Nodeable objects
type FunctionNodeable interface {
	Node() FunctionNode
	DirentType() fuse.DirentType
}

// Dir represents a directory in the filesystem. It contains subnodes of type Dir or FunctionNode
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

func (d Dir) Node() FunctionNode {
	return d
}

func (Dir) DirentType() fuse.DirentType {
	return fuse.DT_Dir
}

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

func (Dir) ReadAll(ctx context.Context) ([]byte, error) {
	return nil, fuse.EPERM
}

func (Dir) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	return fuse.EPERM
}

var _ FunctionNodeable = (*Dir)(nil)

// SliceDir exposes the elements of a slice through the slice's indexes. If a
// slice has length 5, then a slicedir will create nodes named "0", "1", "2",
// "3", and "4".
type SliceDir struct {
	Dir
	Nodes []FunctionNodeable
}

func NewSliceDir(nodes []FunctionNodeable) *SliceDir {
	return &SliceDir{Nodes: nodes}
}

func (d SliceDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	i, err := strconv.Atoi(name)
	if err != nil || i >= len(d.Nodes) {
		return nil, fuse.ENOENT
	}

	return d.Nodes[i].Node(), nil
}

func (d SliceDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	ret := make([]fuse.Dirent, len(d.Nodes))
	for i := range ret {
		ret[i] = fuse.Dirent{Name: strconv.Itoa(i), Type: d.Nodes[i].DirentType()}
	}

	return ret, nil
}

// NewHttpReqDir returns a Dir that represents the values of a http.Request
// object. By default, these values are readable and writeable.
func NewHttpReqDir(req *http.Request) *Dir {
	d := NewDir()
	d.AddNode("method", NewStringFile(&req.Method))
	d.AddNode("url", NewURLFile(req.URL))
	d.AddNode("proto", NewStringFile(&req.Proto))
	d.AddNode("contentlength", NewInt64File(&req.ContentLength))
	d.AddNode("close", NewBoolFile(&req.Close))
	d.AddNode("host", NewStringFile(&req.Host))
	d.AddNode("requrl", NewStringFile(&req.RequestURI))
	return d
}

// NewHttpRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func NewProxyHttpRespDir(resp *http.Response) *Dir {
	d := NewDir()
	d.AddNode("status", NewStringFile(&resp.Status))
	d.AddNode("statuscode", NewIntFile(&resp.StatusCode))
	d.AddNode("proto", NewStringFile(&resp.Proto))
	d.AddNode("contentlength", NewInt64File(&resp.ContentLength))
	d.AddNode("close", NewBoolFile(&resp.Close))
	d.AddNode("req", NewHttpReqDir(resp.Request))
	return d
}
