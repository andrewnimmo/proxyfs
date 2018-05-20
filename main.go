package main

import "log"

func main() {
	proxy, err := NewProxy("www.reddit.com")
	if err != nil {
		log.Fatal(err)
	}

	go proxy.Mount("/tmp/proxyfs")
	log.Fatal(proxy.ListenAndServe(":8081"))
}
