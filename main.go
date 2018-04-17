package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("Starting backend server on port 8080")
	http.Handle("/static/", http.FileServer(http.Dir("front")))
	http.HandleFunc("/run/", runHandler)
	http.HandleFunc("/", serveTemplate)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type sample struct {
	Content string
}

func serveTemplate(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("front/template/index.html")
	if err != nil {
		fmt.Println(err)
		return
	}
	t := template.Must(template.New("sample").Parse(string(content)))
	s := sample{
		Content: `package main

import "fmt"

func main() {
	fmt.Println("Hello Kid!")
}`,
	}
	w.WriteHeader(http.StatusOK)
	err = t.Execute(w, s)
	if err != nil {
		fmt.Println(err)
		return
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
