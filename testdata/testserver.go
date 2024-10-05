package main

import (
	"io"
	"log"
	"net/http"
	"strings"
)

func handler(w http.ResponseWriter, r *http.Request) {
	method := r.Method

	respBody := "Method was: " + method
	if strings.ToLower(method) == "post" {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		respBody += "\nBody was: " + string(b)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(respBody))
}

func main() {

	log.Fatal(http.ListenAndServe(":8081", http.HandlerFunc(handler)))
}
