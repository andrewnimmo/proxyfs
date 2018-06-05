package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/danielthatcher/fusebox"
	"github.com/elazarl/goproxy"
	"github.com/satori/go.uuid"
)

// Proxy can be used to setup a proxy server and a filesystem which can be used to control it
type Proxy struct {
	Server    *goproxy.ProxyHttpServer
	Scope     *regexp.Regexp
	FS        *fusebox.FS
	IntReq    bool
	IntResp   bool
	Requests  []proxyReq
	Responses []proxyResp
	reqMu     *sync.RWMutex
	respMu    *sync.RWMutex
}

// proxyReq is a wrapper for a http.Request, and a channel used to control intercepting
type proxyReq struct {
	Req     *http.Request
	Forward chan int
	Drop    chan int
	ID      uuid.UUID
}

// proxyResp is a wrapper for a http.Response, and a channel used to control intercepting
type proxyResp struct {
	Resp    *http.Response
	Forward chan int
	Drop    chan int
	ID      uuid.UUID
}

// NewProxy returns a new proxy, compiling the given scope to a regexp
func NewProxy(scope string) (*Proxy, error) {
	r, err := regexp.Compile(scope)
	if err != nil {
		return nil, err
	}

	server := goproxy.NewProxyHttpServer()

	ret := &Proxy{
		Server:    server,
		Scope:     r,
		Requests:  make([]proxyReq, 0),
		Responses: make([]proxyResp, 0),
		reqMu:     &sync.RWMutex{},
		respMu:    &sync.RWMutex{},
	}

	fs, d := fusebox.NewEmptyFS()
	ret.FS = fs
	d.AddNode("scope", fusebox.NewRegexpFile(ret.Scope))

	// Intercept controls
	reqNode := fusebox.NewBoolFile(&ret.IntReq)
	respNode := fusebox.NewBoolFile(&ret.IntResp)
	d.AddNode("intreq", reqNode)
	d.AddNode("intresp", respNode)

	// Responses and requests
	d.AddNode("req", newReqListDir(&ret.Requests))
	d.AddNode("resp", newRespListDir(&ret.Responses))

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
		Forward: make(chan int),
		Drop:    make(chan int),
		ID:      id,
	}

	p.respMu.Lock()
	p.Responses = append(p.Responses, pr)
	p.respMu.Unlock()

	// Wait until forwarded
	if p.IntResp {
		select {
		case <-pr.Forward:
		case <-pr.Drop:
			r = droppedResponse(r.Request)
		}
	}

	// Remove the response from the queue before returning
	p.respMu.Lock()
	for i, x := range p.Responses {
		if x.ID == pr.ID {
			p.Responses = append(p.Responses[:i], p.Responses[i+1:]...)
			break
		}
	}
	p.respMu.Unlock()

	return r
}

// HandleRequest handles a request through the proxy server
func (p *Proxy) HandleRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	// Add to the queue
	id, err := uuid.NewV1()
	if err != nil {
		panic("Couldn't create UUID!")
	}
	pr := proxyReq{
		Req:     r,
		Forward: make(chan int),
		Drop:    make(chan int),
		ID:      id,
	}

	p.reqMu.Lock()
	p.Requests = append(p.Requests, pr)
	p.reqMu.Unlock()

	// Wait until forwarded
	var resp *http.Response
	if p.IntReq {
		select {
		case <-pr.Forward:
		case <-pr.Drop:
			resp = droppedResponse(r)
		}
	}

	// Remove the request from the queue before returning
	p.reqMu.Lock()
	for i, x := range p.Requests {
		if x.ID == pr.ID {
			p.Requests = append(p.Requests[:i], p.Requests[i+1:]...)
		}
	}
	p.reqMu.Unlock()

	return r, resp
}

// Mount monuts the proxy's pseudo filesystem at the given path, returning any error encountered.
func (p *Proxy) Mount(path string) error {
	return p.FS.Mount(path)
}

// Listend for changes to p.InterceptRequests and p.InterceptResponses, and start/stop
// intercepting appropriately
func (p *Proxy) dispatchIntercepts(req <-chan int, resp <-chan int) {
	for {
		select {
		case <-req:
			if !p.IntReq {
				for _, r := range p.Requests {
					r.Forward <- 1
				}
			}
		case <-resp:
			if !p.IntResp {
				for _, r := range p.Responses {
					r.Forward <- 1
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
