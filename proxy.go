package proxyfs

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
	Server             *goproxy.ProxyHttpServer
	Scope              *regexp.Regexp
	RootDir            *Dir
	FuseConn           *fuse.Conn
	InterceptResponses bool
	InterceptRequests  bool
}

// NewProxy returns a new proxy, compiling the given scope to a regexp
func NewProxy(scope string) (*Proxy, error) {
	r, err := regexp.Compile(scope)
	if err != nil {
		return nil, err
	}

	server := goproxy.NewProxyHttpServer()

	dir := NewDir()
	ret := &Proxy{Server: server, RootDir: dir, Scope: r}
	dir.AddNode("scope", NewRegexpFile(ret.Scope))
	dir.AddNode("intercept_responses", NewBoolFile(&ret.InterceptResponses))
	dir.AddNode("intercept_requests", NewBoolFile(&ret.InterceptRequests))

	return ret, nil
}

// ListenAndServe sets up the proxy on the given host string (e.g. "127.0.0.1:8080" or ":8080") and
// sets up intercepting functions for in scope items
func (p *Proxy) ListenAndServe(host string) error {
	p.Server.OnRequest(goproxy.UrlMatches(p.Scope)).HandleConnect(goproxy.AlwaysMitm)
	p.Server.OnRequest(goproxy.UrlMatches(p.Scope)).DoFunc(p.HandleRequest)
	p.Server.OnResponse(goproxy.UrlMatches(p.Scope)).DoFunc(p.HandleResponse)

	return http.ListenAndServe(host, p.Server)
}

// HandleResponse handles a response through the proxy server
func (p *Proxy) HandleResponse(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	log.Println("Host:", ctx.Req.Host)
	log.Println("Scope:", p.Scope)
	log.Println("Intercepting requests:", p.InterceptRequests)
	log.Println("Intercepting responses:", p.InterceptResponses)
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
