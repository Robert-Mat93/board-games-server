package main

import (
	"crypto/tls"
	"log"
	"net/http"
)

func main() {
	server := NewServer()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	log.Fatal("HTTP server error: ", http.ListenAndServeTLS(":5000", "ca.pem", "key.pem", server))

}
