package main

import (
	"net/http"

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
	d.AddNode("contentlength", fusebox.NewInt64File(&req.ContentLength))
	d.AddNode("close", fusebox.NewBoolFile(&req.Close))
	d.AddNode("host", fusebox.NewStringFile(&req.Host))
	d.AddNode("requrl", fusebox.NewStringFile(&req.RequestURI))
	return d
}

// NewHttpRespDir returns a Dir that represents the values of a http.Response
// object. By default, these values are readable and writeable.
func NewProxyHttpRespDir(resp *http.Response) *fusebox.Dir {
	d := fusebox.NewDir()
	d.AddNode("status", fusebox.NewStringFile(&resp.Status))
	d.AddNode("statuscode", fusebox.NewIntFile(&resp.StatusCode))
	d.AddNode("proto", fusebox.NewStringFile(&resp.Proto))
	d.AddNode("contentlength", fusebox.NewInt64File(&resp.ContentLength))
	d.AddNode("close", fusebox.NewBoolFile(&resp.Close))
	d.AddNode("req", fusebox.NewHttpReqDir(resp.Request))
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
