package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func handler(w http.ResponseWriter, r *http.Request) {
	echo(os.Stdout, r)
	echo(w, r)
}

func echo(w io.Writer, r *http.Request) {
	_, err := fmt.Fprintf(w, "RequestURI: %s, RemoteAddr: %s, Host: %s\n", r.RequestURI, r.RemoteAddr, r.Host)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func main() {
	http.HandleFunc("/", handler)
	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 30 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
