package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/danielthatcher/fusebox"
	"github.com/elazarl/goproxy"
	"github.com/satori/go.uuid"
)

// Proxy can be used to setup a proxy server and a filesystem which can be used to control it
type Proxy struct {
	Server             *goproxy.ProxyHttpServer
	Scope              *regexp.Regexp
	RootDir            *fusebox.Dir
	FuseConn           *fuse.Conn
	InterceptRequests  bool
	InterceptResponses bool
	ReqDir             *fusebox.Dir
	RespDir            *fusebox.Dir
	ReqNodes           []fusebox.VarNodeable
	RespNodes          []fusebox.VarNodeable
	Requests           []proxyReq
	Responses          []proxyResp
	RequestsLock       *sync.RWMutex
	ResponsesLock      *sync.RWMutex
}

// NewProxy returns a new proxy, compiling the given scope to a regexp
func NewProxy(scope string) (*Proxy, error) {
	r, err := regexp.Compile(scope)
	if err != nil {
		return nil, err
	}

	server := goproxy.NewProxyHttpServer()

	dir := fusebox.NewEmptyDir()
	ret := &Proxy{
		Server:        server,
		RootDir:       dir,
		Scope:         r,
		ReqNodes:      make([]fusebox.VarNodeable, 0),
		RespNodes:     make([]fusebox.VarNodeable, 0),
		Requests:      make([]proxyReq, 0),
		Responses:     make([]proxyResp, 0),
		RequestsLock:  &sync.RWMutex{},
		ResponsesLock: &sync.RWMutex{},
	}

	ret.ReqDir = fusebox.NewSliceDir(&ret.ReqNodes)
	ret.RespDir = fusebox.NewSliceDir(&ret.RespNodes)

	dir.AddNode("scope", fusebox.NewRegexpFile(ret.Scope))

	// Intercept controls
	reqNode := fusebox.NewBoolFile(&ret.InterceptRequests)
	respNode := fusebox.NewBoolFile(&ret.InterceptResponses)
	dir.AddNode("intreq", reqNode)
	dir.AddNode("intresp", respNode)

	// Responses and requests
	dir.AddNode("req", ret.ReqDir)
	dir.AddNode("resp", ret.RespDir)

	go ret.dispatchIntercepts(reqNode.Change, respNode.Change)

	return ret, nil
}

// ListenAndServe sets up the proxy on the given host string (e.g. "127.0.0.1:8080" or ":8080") and
// sets up intercepting functions for in scope items
func (p *Proxy) ListenAndServe(host string, upstream *url.URL) error {
	p.Server.OnRequest(goproxy.UrlMatches(p.Scope)).HandleConnect(goproxy.AlwaysMitm)
	p.Server.OnRequest(goproxy.UrlMatches(p.Scope)).DoFunc(p.HandleRequest)
	p.Server.OnResponse(goproxy.UrlMatches(p.Scope)).DoFunc(p.HandleResponse)

	if upstream != nil {
		u := http.ProxyURL(upstream)
		p.Server.Tr.Proxy = u
	}

	return http.ListenAndServe(host, p.Server)
}

// HandleResponse handles a response through the proxy server
func (p *Proxy) HandleResponse(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	// Add to the queue
	id, err := uuid.NewV1()
	if err != nil {
		panic("Couldn't create UUID!")
	}

	pr := proxyResp{Resp: r,
		Wait: make(chan int),
		Drop: make(chan int),
		ID:   id,
	}

	p.ResponsesLock.Lock()
	p.Responses = append(p.Responses, pr)
	p.RespNodes = append(p.RespNodes, newHTTPRespDir(pr.Resp))
	p.ResponsesLock.Unlock()

	// Wait until forwarded
	if p.InterceptResponses {
		select {
		case <-pr.Wait:
		case <-pr.Drop:
			r = droppedResponse(r.Request)
		}
	}

	// Remove the response from the queue before returning
	p.ResponsesLock.Lock()
	for i, x := range p.Responses {
		if x.ID == pr.ID {
			p.Responses = append(p.Responses[:i], p.Responses[i+1:]...)
			p.RespNodes = append(p.RespNodes[:i], p.RespNodes[i+1:]...)
			break
		}
	}
	p.ResponsesLock.Unlock()

	return r
}

// HandleRequest handles a request through the proxy server
func (p *Proxy) HandleRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	// Add to the queue
	id, err := uuid.NewV1()
	if err != nil {
		panic("Couldn't create UUID!")
	}
	pr := proxyReq{Req: r,
		Wait: make(chan int),
		Drop: make(chan int),
		ID:   id,
	}

	p.RequestsLock.Lock()
	p.Requests = append(p.Requests, pr)
	p.ReqNodes = append(p.ReqNodes, newHTTPReqDir(pr.Req))
	p.RequestsLock.Unlock()

	// Wait until forwarded
	var resp *http.Response
	if p.InterceptRequests {
		select {
		case <-pr.Wait:
		case <-pr.Drop:
			resp = droppedResponse(r)
		}
	}

	// Remove the request from the queue before returning
	p.RequestsLock.Lock()
	for i, x := range p.Requests {
		if x.ID == pr.ID {
			p.Requests = append(p.Requests[:i], p.Requests[i+1:]...)
			p.ReqNodes = append(p.ReqNodes[:i], p.ReqNodes[i+1:]...)
		}
	}
	p.RequestsLock.Unlock()

	return r, resp
}

var _ fs.FS = (*Proxy)(nil)

// Root is implemented to comply with the fs.FS interface.
// It returns a root node of the virtual and an error filesystem
func (p *Proxy) Root() (fs.Node, error) {
	return p.RootDir, nil
}

// Mount monuts the proxy's pseudo filesystem at the given path, returning any error encountered.
func (p *Proxy) Mount(path string) error {
	c, err := fuse.Mount(path, fuse.FSName("proxyfs"))
	if err != nil {
		return err
	}
	p.FuseConn = c

	err = fs.Serve(p.FuseConn, p)
	if err != nil {
		return err
	}

	<-p.FuseConn.Ready
	if err = c.MountError; err != nil {
		return err
	}

	return nil
}

// Listend for changes to p.InterceptRequests and p.InterceptResponses, and start/stop
// intercepting appropriately
func (p *Proxy) dispatchIntercepts(req <-chan int, resp <-chan int) {
	for {
		select {
		case <-req:
			if !p.InterceptRequests {
				for _, r := range p.Requests {
					r.Wait <- 1
				}
			}
		case <-resp:
			if !p.InterceptResponses {
				for _, r := range p.Responses {
					r.Wait <- 1
				}
			}
		}
	}
}

// Create the response returned when a request or response is dropped.
func droppedResponse(req *http.Request) *http.Response {
	msg := "Dropped by proxyfs"
	b := ioutil.NopCloser(bytes.NewBufferString(msg))
	return &http.Response{
		Status:        "500 Internal Server Error",
		StatusCode:    http.StatusInternalServerError,
		Body:          b,
		Header:        make(map[string][]string, 0),
		ContentLength: int64(len(msg)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Close:         true,
		Request:       req,
	}
}
