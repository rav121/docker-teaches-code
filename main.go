package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("Starting backend server on port 8080")
	http.Handle("/", http.FileServer(http.Dir("html")))
	http.HandleFunc("/run", runHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	dir, err := ioutil.TempDir("/tmp/dtc", "dtc-golang-")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	err = ioutil.WriteFile(filepath.Join(dir, "main.go"), buf, 0666)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	cmd := exec.Command("docker", "run", "--rm", "-v", dir+":/dtc", "dtc-golang")
	output := bytes.Buffer{}
	cmd.Stderr = &output
	cmd.Stdout = &output
	err = cmd.Run()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, output.String())
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, output.String())
}
