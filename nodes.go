package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"

	"bazil.org/fuse"
	"github.com/danielthatcher/fusebox"
	"github.com/satori/go.uuid"
)

type reqDirElement struct {
	Data  *http.Request
	files []string
	dirs  []string
}

func newReqDirElement(req *http.Request) *reqDirElement {
	return &reqDirElement{
		Data:  req,
		files: []string{"method", "url", "proto", "close", "host", "raw", "contentlength", "body"},
		dirs:  []string{"headers"},
	}
}

func (e *reqDirElement) GetNode(ctx context.Context, k string) (fusebox.VarNode, error) {
	switch k {
	case "method":
		return fusebox.NewStringFile(&e.Data.Method), nil
	case "url":
		return fusebox.NewURLFile(e.Data.URL), nil
	case "requrl":
		return fusebox.NewStringFile(&e.Data.RequestURI), nil
	case "proto":
		return fusebox.NewStringFile(&e.Data.Proto), nil
	case "close":
		return fusebox.NewBoolFile(&e.Data.Close), nil
	case "host":
		return fusebox.NewStringFile(&e.Data.Host), nil
	case "headers":
		d := newHTTPHeaderDir(&e.Data.Header)
		d.OpenFlags = fuse.OpenDirectIO
		return d, nil
	case "raw":
		return newHTTPReqRawFile(e.Data), nil
	case "contentlength":
		return fusebox.NewInt64File(&e.Data.ContentLength), nil
	case "body":
		return newHTTPBodyFile(&e.Data.Body), nil
	}

	return nil, fuse.ENOENT
}

func (e *reqDirElement) GetDirentType(ctx context.Context, k string) (fuse.DirentType, error) {
	for _, v := range e.dirs {
		if k == v {
			return fuse.DT_Dir, nil
		}
	}

	for _, v := range e.files {
		if k == v {
			return fuse.DT_File, nil
		}
	}

	return fuse.DT_Unknown, fuse.ENOENT
}

func (e *reqDirElement) GetKeys(ctx context.Context) []string {
	return append(e.files, e.dirs...)
}

func (*reqDirElement) AddNode(name string, node interface{}) error {
	return fuse.EPERM
}

func (*reqDirElement) RemoveNode(name string) error {
	return fuse.EPERM
}

// newHTTPReqDir returns a Dir that represents the values of a http.Request
// object. By default, these values are readable and writeable.
func newHTTPReqDir(req *http.Request) *fusebox.Dir {
	ret := fusebox.NewDir(newReqDirElement(req))
	ret.Mode = os.ModeDir | 0444
	return ret
}

type respDirElement struct {
	Data  *http.Response
	files []string
	dirs  []string
}

func newRespDirElement(resp *http.Response) *respDirElement {
	return &respDirElement{
		Data:  resp,
		files: []string{"status", "statuscode", "proto", "close", "raw", "contentlength", "body"},
		dirs:  []string{"headers", "req"},
	}
}

func (e *respDirElement) GetNode(ctx context.Context, k string) (fusebox.VarNode, error) {
	switch k {
	case "status":
		return fusebox.NewStringFile(&e.Data.Status), nil
	case "statuscode":
		return fusebox.NewIntFile(&e.Data.StatusCode), nil
	case "proto":
		return fusebox.NewStringFile(&e.Data.Proto), nil
	case "close":
		return fusebox.NewBoolFile(&e.Data.Close), nil
	case "headers":
		return newHTTPHeaderDir(&e.Data.Header), nil
	case "req":
		return newHTTPReqDir(e.Data.Request), nil
	case "raw":
		return newHTTPRespRawFile(e.Data), nil
	case "contentlength":
		return fusebox.NewInt64File(&e.Data.ContentLength), nil
	case "body":
		return newHTTPBodyFile(&e.Data.Body), nil
	}

	return nil, fuse.ENOENT
}

func (e *respDirElement) GetDirentType(ctx context.Context, k string) (fuse.DirentType, error) {
	for _, v := range e.dirs {
		if k == v {
			return fuse.DT_Dir, nil
		}
	}

	for _, v := range e.files {
		if k == v {
			return fuse.DT_File, nil
		}
	}

	return fuse.DT_Unknown, fuse.ENOENT
}

func (e *respDirElement) GetKeys(ctx context.Context) []string {
	return append(e.files, e.dirs...)
}

func (*respDirElement) AddNode(name string, node interface{}) error {
	return fuse.EPERM
}

func (*respDirElement) RemoveNode(name string) error {
	return fuse.EPERM
}

// newHTTPRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func newHTTPRespDir(resp *http.Response) *fusebox.Dir {
	ret := fusebox.NewDir(newRespDirElement(resp))
	ret.Mode = os.ModeDir | 0444
	return ret
}

// proxyReq is a wrapper for a http.Request, and a channel used to control intercepting
type proxyReq struct {
	Req  *http.Request
	Wait chan int
	Drop chan int
	ID   uuid.UUID
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
	// A pointer to the actual Request or Response's body
	Body *io.ReadCloser
}

// Returns a new HTTPBodyFile that exposes and updates the given body, as well as
// automatically updating the given content length.
func newHTTPBodyFile(body *io.ReadCloser) *fusebox.File {
	return fusebox.NewFile(&httpBodyFile{body})
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

type headerElement struct {
	Data *http.Header
}

func (e *headerElement) GetNode(ctx context.Context, k string) (fusebox.VarNode, error) {
	h, ok := (*e.Data)[k]
	if !ok {
		return nil, fuse.ENOENT
	}
	ret := fusebox.NewStringFile(&h[0])
	ret.OpenFlags = fuse.OpenDirectIO
	return ret, nil
}

func (e *headerElement) GetDirentType(ctx context.Context, k string) (fuse.DirentType, error) {
	_, ok := (*e.Data)[k]
	if !ok {
		return fuse.DT_Unknown, fuse.ENOENT
	}

	return fuse.DT_File, nil
}

func (e *headerElement) GetKeys(ctx context.Context) []string {
	ret := make([]string, len(*e.Data))
	i := 0
	for k := range *e.Data {
		ret[i] = k
		i++
	}

	return ret
}
func (e *headerElement) AddNode(name string, node interface{}) error {
	return fuse.EPERM
}
func (e *headerElement) RemoveNode(name string) error {
	return fuse.EPERM
}

// Returns a new Dir that exposes the headers of a request or response, with
// the name of the contained files being the header names, and their contents
// being the header values. For now this is limited to just the first string
// for a given key in http.Header
func newHTTPHeaderDir(h *http.Header) *fusebox.Dir {
	ret := fusebox.NewDir(&headerElement{h})
	ret.Mode = os.ModeDir | 0444
	return ret
}

// A file that exposes a HTTP requests in its raw format for reading and editing.
// For limitations on reading, see
// https://godoc.org/net/http/httputil#DumpRequest
type httpReqRawFile struct {
	Data *http.Request
}

// Return a HTTPReqRawFile for the given http.Request.
func newHTTPReqRawFile(req *http.Request) *fusebox.File {
	ret := fusebox.NewFile(&httpReqRawFile{Data: req})
	ret.OpenFlags = fuse.OpenDirectIO
	return ret
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
