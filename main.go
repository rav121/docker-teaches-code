package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

func main() {
	fmt.Println("Parsing languages")
	if err := parseLanguages(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Starting backend server on port 8080")
	http.Handle("/static/", http.FileServer(http.Dir("front")))
	http.HandleFunc("/run/", runHandler)
	http.HandleFunc("/", serveTemplate)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type lang struct {
	Name    string `json:",omitempty"`
	File    string
	Content string
}

var languages = []lang{}

func parseLanguages() error {
	err := filepath.Walk("lang", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == "lang" {
			return nil
		}
		if info.IsDir() {
			l := lang{Name: filepath.Base(path)}
			data, err := ioutil.ReadFile(filepath.Join(path, "config.json"))
			if err != nil {
				return err
			}
			err = json.Unmarshal(data, &l)
			if err != nil {
				return err
			}
			content, err := ioutil.ReadFile(filepath.Join(path, "sample"))
			if err != nil {
				return err
			}
			l.Content = string(content)
			languages = append(languages, l)
		}
		return filepath.SkipDir
	})
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Name < languages[j].Name
	})
	return err
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
	w.WriteHeader(http.StatusOK)
	err = t.Execute(w, languages[0])
	if err != nil {
		fmt.Println(err)
		return
	}
}

type request struct {
	Lang string
	Code string
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	req := request{}
	err = json.Unmarshal(data, &req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	s, err := build(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, s)
}

func findLanguage(language string) (lang, error) {
	for _, l := range languages {
		if l.Name == language {
			return l, nil
		}
	}
	return lang{}, fmt.Errorf("invalid language '%s'", language)
}

func build(req request) (string, error) {
	lang, err := findLanguage(req.Lang)
	if err != nil {
		return "", err
	}
	dir, err := ioutil.TempDir("/tmp/dtc", "dtc-"+req.Lang+"-")
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(filepath.Join(dir, lang.File), []byte(req.Code), 0666)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("docker", "run", "--rm", "-v", dir+":/dtc", "dtc-"+req.Lang)
	output := bytes.Buffer{}
	cmd.Stderr = &output
	cmd.Stdout = &output
	cmd.Run()
	return output.String(), nil
}
