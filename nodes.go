package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"

	"bazil.org/fuse"
	"github.com/danielthatcher/fusebox"
	"github.com/satori/go.uuid"
)

// NewHTTPReqDir returns a Dir that represents the values of a http.Request
// object. By default, these values are readable and writeable.
func NewHTTPReqDir(req *http.Request) *fusebox.Dir {
	d := fusebox.NewEmptyDir()
	d.AddNode("method", fusebox.NewStringFile(&req.Method))
	d.AddNode("url", fusebox.NewURLFile(req.URL))
	d.AddNode("proto", fusebox.NewStringFile(&req.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&req.Close))
	d.AddNode("host", fusebox.NewStringFile(&req.Host))
	d.AddNode("headers", newHTTPHeaderDir(req.Header))

	r := newHTTPReqRawFile(req)
	d.AddNode("raw", r)
	/*
		go func() {
			for {
				<-r.Change
				r.Lock.RLock()
				refreshHeaders(d, r.Elem.Data.Header)
				r.Lock.RUnlock()
			}
		}()*/

	d.AddNode("requrl", fusebox.NewStringFile(&req.RequestURI))

	lenNode := fusebox.NewInt64File(&req.ContentLength)
	d.AddNode("contentlength", lenNode)
	d.AddNode("body", newHTTPBodyFile(&req.Body, &req.ContentLength))

	// Allow removing with "rm -r"
	// TODO: allow removing
	return d
}

// NewHTTPRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func NewHTTPRespDir(resp *http.Response) *fusebox.Dir {
	d := fusebox.NewEmptyDir()
	d.AddNode("status", fusebox.NewStringFile(&resp.Status))
	d.AddNode("statuscode", fusebox.NewIntFile(&resp.StatusCode))
	d.AddNode("proto", fusebox.NewStringFile(&resp.Proto))
	d.AddNode("close", fusebox.NewBoolFile(&resp.Close))
	d.AddNode("headers", newHTTPHeaderDir(resp.Header))
	d.AddNode("req", NewHTTPReqDir(resp.Request))

	r := newHTTPRespRawFile(resp)
	d.AddNode("raw", r)
	/*
		go func() {
			for {
				<-r.Change
				r.Lock.RLock()
				refreshHeaders(d, r.Data.Header)
				r.Lock.RUnlock()
			}
		}()*/

	lenNode := fusebox.NewInt64File(&resp.ContentLength)
	d.AddNode("contentlength", lenNode)
	d.AddNode("body", newHTTPBodyFile(&resp.Body, &resp.ContentLength))

	// Allow removing with "rm -r"
	// TODO: allow removing
	return d
}

func refreshHeaders(d *fusebox.Dir, h http.Header) {
	d.RemoveNode("headers")
	d.AddNode("headers", newHTTPHeaderDir(h))
}

// proxyReq is a wrapper for a http.Request, and a channel used to control intercepting
type proxyReq struct {
	Req  *http.Request
	Wait chan int
	Drop chan int
	ID   uuid.UUID
}

func (p *proxyReq) Node() fusebox.VarNode {
	return NewHTTPReqDir(p.Req)
}

func (proxyReq) DirentType() fuse.DirentType {
	return fuse.DT_Dir
}

// proxyResp is a wrapper for a http.Response, and a channel used to control intercepting
type proxyResp struct {
	Resp *http.Response
	Wait chan int
	Drop chan int
	ID   uuid.UUID
}

// Provides a node for reading a writing the http body, and updating the content length
// to match the body.
type httpBodyFile struct {
	// Stored so that the content length can be updated when the body changes.
	ContentLength *int64

	// A pointer to the actual Request or Response's body
	Body *io.ReadCloser
}

// Returns a new HTTPBodyFile that exposes and updates the given body, as well as
// automatically updating the given content length.
func newHTTPBodyFile(body *io.ReadCloser, length *int64) *fusebox.File {
	return fusebox.NewFile(&httpBodyFile{ContentLength: length, Body: body})
}

// Read a copy of the body, and replace the original reader with a fresh one to allow
// for future reading.
func (bf *httpBodyFile) readCopy() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	tee := io.TeeReader(*bf.Body, buf)
	data, err := ioutil.ReadAll(tee)
	*bf.Body = ioutil.NopCloser(buf)

	return data, err
}

func (bf *httpBodyFile) ValRead(ctx context.Context) ([]byte, error) {
	data, err := bf.readCopy()
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (bf *httpBodyFile) ValWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// Update the data
	b := bytes.TrimSpace(req.Data)
	*bf.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	*bf.ContentLength = int64(len(b))

	resp.Size = len(req.Data)
	return nil
}

func (bf *httpBodyFile) Size(context.Context) (uint64, error) {
	b, err := bf.readCopy()
	if err != nil {
		return 0, err
	}
	return uint64(len(b)), nil
}

// Returns a new Dir that exposes the headers of a request or response, with
// the name of the contained files being the header names, and their contents
// being the header values. For now this is limited to just the first string
// for a given key in http.Header
func newHTTPHeaderDir(h http.Header) *fusebox.Dir {
	d := fusebox.NewEmptyDir()
	for k := range h {
		d.AddNode(k, fusebox.NewStringFile(&h[k][0]))
	}

	return d
}

// A file that exposes a HTTP requests in its raw format for reading and editing.
// For limitations on reading, see
// https://godoc.org/net/http/httputil#DumpRequest
type httpReqRawFile struct {
	Data *http.Request
}

// Return a HTTPReqRawFile for the given http.Request.
func newHTTPReqRawFile(req *http.Request) *fusebox.File {
	return fusebox.NewFile(&httpReqRawFile{Data: req})
}

func (rf *httpReqRawFile) ValRead(ctx context.Context) ([]byte, error) {
	data, err := httputil.DumpRequest(rf.Data, true)
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (rf *httpReqRawFile) ValWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	buf := bufio.NewReader(bytes.NewReader(req.Data))
	httpReq, err := http.ReadRequest(buf)
	if err != nil {
		return fuse.ERANGE
	}

	*rf.Data = *httpReq
	resp.Size = len(req.Data)
	return nil
}

func (rf *httpReqRawFile) Size(context.Context) (uint64, error) {
	data, err := httputil.DumpRequest(rf.Data, true)
	if err != nil {
		return 0, fuse.ENODATA
	}

	return uint64(len(data)), nil
}

// A file that exposes a HTTP response in it's raw format. The reading limitations
// are the same as those for HTTPReqRawFile, which come from
// https://godoc.org/net/http/httputil#DumpRequest
type httpRespRawFile struct {
	Data *http.Response
}

// Return a new HTTPRespRawFile for the given http.Response
func newHTTPRespRawFile(resp *http.Response) *fusebox.File {
	return fusebox.NewFile(&httpRespRawFile{Data: resp})
}

func (rf *httpRespRawFile) ValRead(ctx context.Context) ([]byte, error) {
	data, err := httputil.DumpResponse(rf.Data, true)
	if err != nil {
		return nil, fuse.ENODATA
	}

	return data, nil
}

func (rf *httpRespRawFile) ValWrite(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	buf := bufio.NewReader(bytes.NewReader(req.Data))
	httpResp, err := http.ReadResponse(buf, rf.Data.Request)
	if err != nil {
		return fuse.ERANGE
	}

	*rf.Data = *httpResp
	resp.Size = len(req.Data)
	return nil
}

func (rf *httpRespRawFile) Size(context.Context) (uint64, error) {
	data, err := httputil.DumpResponse(rf.Data, true)
	if err != nil {
		return 0, fuse.ENODATA
	}

	return uint64(len(data)), nil
}
