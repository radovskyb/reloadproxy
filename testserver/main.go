package main

import (
	"fmt"
	"log"
	"net/http"

	_ "github.com/radovskyb/reloadproxy"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!")
	})
	http.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "a")
	})
	http.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "b")
	})
	log.Fatal(http.ListenAndServe(":9000", nil))
}
