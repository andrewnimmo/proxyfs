package main

import (
	"log"
	"net/http"
	"regexp"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/elazarl/goproxy"
)

// Proxy can be used to setup a proxy server and a filesystem which can be used to control it
type Proxy struct {
	Server  *goproxy.ProxyHttpServer
	Scope   string
	RootDir *Dir
}

// NewProxy returns a new proxy, compiling the given scope to a regexp
func NewProxy(scope string) (*Proxy, error) {
	_, err := regexp.Compile(scope)
	if err != nil {
		return nil, err
	}

	server := goproxy.NewProxyHttpServer()

	dir := NewDir()
	ret := &Proxy{Scope: scope, Server: server, RootDir: dir}
	dir.AddNode("scope", NewStringFuncRW(&ret.Scope))

	return ret, nil
}

// ListenAndServe sets up the proxy on the given host string (e.g. "127.0.0.1:8080" or ":8080") and
// sets up intercepting functions for in scope items
func (p *Proxy) ListenAndServe(host string) error {
	scopeRegexp := regexp.MustCompile(p.Scope)
	p.Server.OnRequest(goproxy.ReqHostMatches(scopeRegexp)).HandleConnect(goproxy.AlwaysMitm)
	p.Server.OnResponse(goproxy.ReqHostMatches(scopeRegexp)).DoFunc(p.HandleResponse)

	return http.ListenAndServe(host, p.Server)
}

// HandleResponse handles a response through the proxy server
func (p *Proxy) HandleResponse(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	log.Println(ctx.Req.Host)
	r.Header.Set("Proxied", "true")

	return r
}

// HandleRequest handles a request through the proxy server
func (p *Proxy) HandleRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

var _ fs.FS = (*Proxy)(nil)

// Root is implemented to comply with the fs.FS interface. It returns a root node of the virtual and an error filesystem
func (p *Proxy) Root() (fs.Node, error) {
	return p.RootDir, nil
}

func (p *Proxy) Mount(path string) (*fuse.Conn, error) {
	c, err := fuse.Mount(path, fuse.FSName("proxyfs"))
	if err != nil {
		defer c.Close()
		return nil, err
	}

	err = fs.Serve(c, p)
	if err != nil {
		defer c.Close()
		return nil, err
	}

	<-c.Ready
	if err = c.MountError; err != nil {
		defer c.Close()
		return nil, err
	}

	return c, nil
}
