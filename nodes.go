package main

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"bazil.org/fuse"
	"github.com/danielthatcher/fusebox"
	"github.com/gobuffalo/uuid"
)

// NewHttpReqDir returns a Dir that represents the values of a http.Request
// object. By default, these values are readable and writeable.
func NewHttpReqDir(req *http.Request) *fusebox.Dir {
	d := fusebox.NewDir()
	d.AddNode("method", fusebox.NewStringFile(&req.Method))
	d.AddNode("url", fusebox.NewURLFile(req.URL))
	d.AddNode("proto", fusebox.NewStringFile(&req.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&req.Close))
	d.AddNode("host", fusebox.NewStringFile(&req.Host))

	reqNode := fusebox.NewStringFile(&req.RequestURI)
	reqNode.Mode = os.ModeDir | 04444
	d.AddNode("requrl", reqNode)

	lenNode := fusebox.NewInt64File(&req.ContentLength)
	d.AddNode("contentlength", lenNode)
	d.AddNode("body", NewHTTPBodyFile(&req.Body, lenNode))
	return d
}

// NewHttpRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func NewProxyHttpRespDir(resp *http.Response) *fusebox.Dir {
	d := fusebox.NewDir()
	d.AddNode("status", fusebox.NewStringFile(&resp.Status))
	d.AddNode("statuscode", fusebox.NewIntFile(&resp.StatusCode))
	d.AddNode("proto", fusebox.NewStringFile(&resp.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&resp.Close))
	d.AddNode("req", NewHttpReqDir(resp.Request))

	lenNode := fusebox.NewInt64File(&resp.ContentLength)
	d.AddNode("contentlength", lenNode)
	d.AddNode("body", NewHTTPBodyFile(&resp.Body, lenNode))
	return d
}

// ProxyReq is a wrapper for a http.Request, and a channel used to control intercepting
type ProxyReq struct {
	Req  *http.Request
	Wait chan int
	ID   uuid.UUID
}

// ProxyResp is a wrapper for a http.Response, and a channel used to control intercepting
type ProxyResp struct {
	Resp *http.Response
	Wait chan int
	ID   uuid.UUID
}

// A slice of ProxyReq that implements VarNodeSliceable
type ProxyRequests []ProxyReq

// A slice of ProxyResp that implements VarNodeSliceable
type ProxyResponses []ProxyResp

// Interface implementations for ProxyReq and ProxyResp...

func (pr ProxyRequests) GetNode(i int) fusebox.VarNode {
	return NewHttpReqDir(pr[i].Req)
}

func (ProxyRequests) GetDirentType(i int) fuse.DirentType {
	return fuse.DT_Dir
}

func (pr ProxyRequests) Length() int {
	return len(pr)
}

func (pr ProxyResponses) GetNode(i int) fusebox.VarNode {
	return NewProxyHttpRespDir(pr[i].Resp)
}

func (ProxyResponses) GetDirentType(i int) fuse.DirentType {
	return fuse.DT_Dir
}

func (pr ProxyResponses) Length() int {
	return len(pr)
}

// Provides a node for reading a writing the http body, and updating the content length
// to match the body.
type HTTPBodyFile struct {
	fusebox.File

	// Stored so that the content length can be updated when the body changes. The node
	// is required so that the lock can be used for safe access.
	ContentLengthFile *fusebox.Int64File

	// A pointer to the actual Request or Response's body
	Body *io.ReadCloser
}

// Returns a new HTTPBodyFile that exposes and updates the given body, as well as
// automatically updating the given content length.
func NewHTTPBodyFile(body *io.ReadCloser, length *fusebox.Int64File) *HTTPBodyFile {
	ret := &HTTPBodyFile{Body: body, ContentLengthFile: length}
	ret.Lock = &sync.RWMutex{}
	ret.Mode = 0666
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

// Read a copy of the body, and replace the original reader with a fresh one to allow
// for future reading.
func (bf *HTTPBodyFile) readCopy() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	tee := io.TeeReader(*bf.Body, buf)
	data, err := ioutil.ReadAll(tee)
	*bf.Body = ioutil.NopCloser(buf)

	return data, err
}

func (bf *HTTPBodyFile) valRead(ctx context.Context) ([]byte, error) {
	data, err := bf.readCopy()
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (bf *HTTPBodyFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// Update the data
	b := bytes.TrimSpace(req.Data)
	*bf.Body = ioutil.NopCloser(bytes.NewBuffer(b))

	// Update the content length
	bf.ContentLengthFile.Lock.Lock()
	defer bf.ContentLengthFile.Lock.Unlock()
	*bf.ContentLengthFile.Data = int64(len(b))

	resp.Size = len(req.Data)
	return nil
}

func (bf *HTTPBodyFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = bf.Mode

	// The content length for a response is sometimes -1, so we have to read
	// the whole thing to get the length
	data, err := bf.readCopy()
	if err != nil {
		return fuse.ENODATA
	}

	attr.Size = uint64(len(data))

	return nil
}

func (bf *HTTPBodyFile) Node() fusebox.VarNode {
	return bf
}
