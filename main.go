package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"

	"bazil.org/fuse"
	flag "github.com/spf13/pflag"
)

func main() {
	// Flag parsing
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s [OPTIONS]... [MOUNTPOINT]\n", os.Args[0])
		flag.PrintDefaults()
	}
	bindHost := flag.IPP("listen", "l", net.ParseIP("127.0.0.1"), "The address to listen on. Defaults to loopback interface.")
	bindPort := flag.IntP("port", "p", 8080, "The port to listen on.")
	scope := flag.StringP("scope", "s", ".", "A regex defining the scope of what to intercept.")
	upstream := flag.StringP("upstream", "u", "", "The address of the upstream proxy to use.")
	flag.Parse()

	if flag.NArg() != 1 || flag.Arg(0) == "" {
		fmt.Println("Please supply a mountpoint!")
		flag.Usage()
		os.Exit(1)
	}

	mountpoint := flag.Arg(0)

	// Validate arguments
	var upURL *url.URL
	if *upstream != "" {
		u, err := url.Parse(*upstream)
		if err != nil {
			log.Fatal(err)
		}

		upURL = u
	}

	// Run the proxy and filesystem
	proxy, err := NewProxy(*scope)
	if err != nil {
		log.Fatal(err)
	}

	// Handle ctrl-c
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		if err := fuse.Unmount(mountpoint); err != nil {
			log.Printf("Failed to properly unmount: %v\n", err)
		}
		os.Exit(1)
	}()

	// Actually run
	go func() {
		if err := proxy.Mount(mountpoint); err != nil {
			log.Fatalf("Failed to mount: %v\n", err)
		}
	}()

	bind := fmt.Sprintf("%v:%v", *bindHost, *bindPort)
	log.Fatal(proxy.ListenAndServe(bind, upURL))
}
