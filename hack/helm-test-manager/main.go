package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	flag.String("config", "", "Ignored controller-manager configuration path")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", ok)
	mux.HandleFunc("/readyz", ok)
	log.Fatal(http.ListenAndServe(":8081", mux))
}

func ok(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}
