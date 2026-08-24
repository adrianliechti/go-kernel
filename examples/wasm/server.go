//go:build !js

package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	dir := flag.String("dir", "dist", "directory to serve")
	flag.Parse()

	log.Printf("serving %s at http://%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))
}
