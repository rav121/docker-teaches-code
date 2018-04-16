package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	fmt.Println("Starting backend server on port 8080")
	http.Handle("/", http.FileServer(http.Dir("html")))
	http.HandleFunc("/run", runHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(buf))
}
