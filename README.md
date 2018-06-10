# ProxyFS
ProyxFS is a HTTP proxy that exposes requests, responses, and controls through a FUSE file system. It is not intended to provide a permanent solution to any problem, though by exposing controls and data as files which can be operated on by a wide range of tools, it does allow for functionality to be hacked together quickly in situations where time is limited.

## Installing
The simplest way to install is to download a binary from the [releases](https://github.com/danielthatcher/proxyfs/releases) page. Both Linux and OSX are supported, however, the code has not been tested on OSX.

Alternatively, if you have a properly configured [Go environment](https://golang.org/doc/install), you can install from source using:

```
go get -u -v github.com/danielthatcher/proxyfs
```

## Usage
ProxyFS can be started as follows, where `mountpoint` is an empty directory
```
proxyfs <mountpoint>
```

The help text is shown below:
```
$ proxyfs -h

Usage of proxyfs:
proxyfs [OPTIONS]... [MOUNTPOINT]
  -l, --listen ip         The address to listen on. Defaults to loopback interface. (default 127.0.0.1)
  -p, --port int          The port to listen on. (default 8080)
  -s, --scope string      A regex defining the scope of what to intercept. (default ".")
  -u, --upstream string   The address of the upstream proxy to use.
pflag: help requested
```
### Files
Once running, a file structure such as the one below will be created in the mount point:
```
.
├── intreq
├── intresp
├── req
├── resp
├── scope
├── urlreq
└── urlresp
```

These files have the following roles:
* `intreq` and `intresp` are boolean nodes (containing a '0' or a '1' for true and false respectively) that control whether requests and responses are being intercepted by the proxy rather than forwarded.
* `req` and `resp` are directories that contain and requests and responses in the queue when intercepting is turned on.
* `scope` is a regular expression to match the URLs of requests and responses that should be intercepted by the proxy.
* `urlreq` and `urlresp` are files that can be continuously read from, and will output the URL of the request/response that is at the top of the request/response queue whenever it changes.

Once intercepting is turned on, and requests or responses are waiting in the queue, the `req` and `resp` directories will be populated with numbered directories with a structure similar to the following:
```
/tmp/proxyfs/req
└── 0
    ├── body
    ├── close
    ├── contentlength
    ├── forward
    ├── headers
    │   ├── Accept
    ...
    │   └── User-Agent
    ├── host
    ├── method
    ├── proto
    ├── raw
    └── url
```

The directory numbered 0 is at the top of the queue. The most notable nodes in this directory are:
* `body` - the body of the request or response
* `headers` - a directory containing the value of each header in a separate file.
* `raw` - the complete request or response in its raw form
* `forward` - any data written to this node will cause the request to be forwarded.

Requests and responses can be dropped by removing their directories, e.g.:
```
rm -r req/0
```

### Demo Script
Below is a demo script that simple prints out the URL for each intercepted request, before forwarding it:

```
#! /bin/bash

if [ -z "$1" ]; then
    echo "Please give the mountput of proxyfs"
    exit 1
fi

# Get the proxy's mount point and start intercepting requests
pfs="$1"
echo 1 >$pfs/intreq

# Wait for new requests, print their URLs before forwarding them.
while read url; do
    echo $url
    echo z > $pfs/req/0/forward
done <$pfs/urlreq
```

While the above script does not do anything useful, it provides a template for running a bash command on each request in scope, which can be useful in a number of scenarios.

## This is Disgusting! Why?
I work as a pentester, and there have been a number of times when testing webapps that I wished I'd had access to certain functionality in an intercepting proxy (e.g. as a BurpSuite plugin), but unfortunately did not. I've noted that a lot of the time I could have implemented this functionality quickly if I had been able to use bash scripts to modify or react to requests and responses.

And yes, this is really hacky. As previously stated, it shouldn't be used in a permanent solution to any problem. However, until you find time to implement something better, it will make do.
