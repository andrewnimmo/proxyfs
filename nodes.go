package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sync"

	"bazil.org/fuse"
	"github.com/danielthatcher/fusebox"
	"github.com/gobuffalo/uuid"
)

// NewHTTPReqDir returns a Dir that represents the values of a http.Request
// object. By default, these values are readable and writeable.
func NewHTTPReqDir(req *http.Request) *fusebox.Dir {
	d := fusebox.NewDir()
	d.AddNode("method", fusebox.NewStringFile(&req.Method))
	d.AddNode("url", fusebox.NewURLFile(req.URL))
	d.AddNode("proto", fusebox.NewStringFile(&req.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&req.Close))
	d.AddNode("host", fusebox.NewStringFile(&req.Host))
	d.AddNode("headers", NewHTTPHeaderDir(req.Header))
	d.AddNode("raw", NewHTTPReqRawFile(req))

	d.AddNode("requrl", fusebox.NewStringFile(&req.RequestURI))

	lenNode := fusebox.NewInt64File(&req.ContentLength)
	d.AddNode("contentlength", lenNode)
	d.AddNode("body", NewHTTPBodyFile(&req.Body, lenNode))
	return d
}

// NewHTTPRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func NewHTTPRespDir(resp *http.Response) *fusebox.Dir {
	d := fusebox.NewDir()
	d.AddNode("status", fusebox.NewStringFile(&resp.Status))
	d.AddNode("statuscode", fusebox.NewIntFile(&resp.StatusCode))
	d.AddNode("proto", fusebox.NewStringFile(&resp.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&resp.Close))
	d.AddNode("headers", NewHTTPHeaderDir(resp.Header))
	d.AddNode("raw", NewHTTPRespRawFile(resp))
	d.AddNode("req", NewHTTPReqDir(resp.Request))

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
	d := NewHTTPReqDir(pr[i].Req)
	d.AddNode("forward", fusebox.NewChanFile(pr[i].Wait))
	return d
}

func (ProxyRequests) GetDirentType(i int) fuse.DirentType {
	return fuse.DT_Dir
}

func (pr ProxyRequests) Length() int {
	return len(pr)
}

func (pr ProxyResponses) GetNode(i int) fusebox.VarNode {
	d := NewHTTPRespDir(pr[i].Resp)
	d.AddNode("forward", fusebox.NewChanFile(pr[i].Wait))
	return d
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

// Returns a new Dir that exposes the headers of a request or response, with
// the name of the contained files being the header names, and their contents
// being the header values. For now this is limited to just the first string
// for a given key in http.Header
func NewHTTPHeaderDir(h http.Header) *fusebox.Dir {
	d := fusebox.NewDir()
	for k := range h {
		d.AddNode(k, fusebox.NewStringFile(&h[k][0]))
	}

	return d
}

// A file that exposes a HTTP requests in its raw format for reading and editing.
// For limitations on reading, see
// https://godoc.org/net/http/httputil#DumpRequest
type HTTPReqRawFile struct {
	fusebox.File
	Data *http.Request
}

// Return a HTTPReqRawFile for the given http.Request.
func NewHTTPReqRawFile(req *http.Request) *HTTPReqRawFile {
	ret := &HTTPReqRawFile{Data: req}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

func (rf *HTTPReqRawFile) valRead(ctx context.Context) ([]byte, error) {
	data, err := httputil.DumpRequest(rf.Data, true)
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (rf *HTTPReqRawFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	buf := bufio.NewReader(bytes.NewReader(req.Data))
	httpReq, err := http.ReadRequest(buf)
	if err != nil {
		return fuse.ERANGE
	}

	*rf.Data = *httpReq
	resp.Size = len(req.Data)
	return nil
}

func (rf *HTTPReqRawFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = rf.Mode

	data, err := httputil.DumpRequest(rf.Data, true)
	if err != nil {
		return fuse.ENODATA
	}

	attr.Size = uint64(len(data))
	return nil
}

func (rf *HTTPReqRawFile) Node() fusebox.VarNode {
	return rf
}

// A file that exposes a HTTP response in it's raw format. The reading limitations
// are the same as those for HTTPReqRawFile, which come from
// https://godoc.org/net/http/httputil#DumpRequest
type HTTPRespRawFile struct {
	fusebox.File
	Data *http.Response
}

// Return a new HTTPRespRawFile for the given http.Response
func NewHTTPRespRawFile(resp *http.Response) *HTTPRespRawFile {
	ret := &HTTPRespRawFile{Data: resp}
	ret.Mode = 0666
	ret.Lock = &sync.RWMutex{}
	ret.ValRead = ret.valRead
	ret.ValWrite = ret.valWrite
	return ret
}

func (rf *HTTPRespRawFile) valRead(ctx context.Context) ([]byte, error) {
	data, err := httputil.DumpResponse(rf.Data, true)
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (rf *HTTPRespRawFile) valWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	buf := bufio.NewReader(bytes.NewReader(req.Data))
	httpResp, err := http.ReadResponse(buf, rf.Data.Request)
	if err != nil {
		return fuse.ERANGE
	}

	*rf.Data = *httpResp
	resp.Size = len(req.Data)
	return nil
}

func (rf *HTTPRespRawFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = rf.Mode
	data, err := httputil.DumpResponse(rf.Data, true)
	if err != nil {
		return fuse.ENODATA
	}

	attr.Size = uint64(len(data))
	return nil
}

func (rf *HTTPRespRawFile) Node() fusebox.VarNode {
	return rf
}
