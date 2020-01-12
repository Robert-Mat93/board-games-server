package main

import (
	"log"
	"net/http"
)

func main() {
	server := NewServer()
	//log.Fatal("HTTP server error: ", http.ListenAndServeTLS(":5000", "ca.pem", "key.pem", server))
	log.Fatal("HTTP server error: ", http.ListenAndServe(":5000", server))

}
