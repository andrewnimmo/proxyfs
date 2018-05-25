package main

import (
	"net/http"

	"github.com/danielthatcher/fusebox"
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
