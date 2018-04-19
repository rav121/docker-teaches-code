package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	http.Handle("/", http.FileServer(http.Dir("front")))
	http.HandleFunc("/run/", runHandler)
	http.HandleFunc("/sample/", sampleHandler)
	http.HandleFunc("/languages/", languagesHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type sample struct {
	Name string `json:"name"`
	File string `json:"file"`
}

type lang struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	File    string   `json:"file"`
	Samples []sample `json:"samples"`
	path    string
}

var languages = []lang{}

func parseLanguages() error {
	root := "lang"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if info.IsDir() {
			data, err := ioutil.ReadFile(filepath.Join(path, "config.json"))
			if err != nil {
				return err
			}
			l := lang{
				ID:   filepath.Base(path),
				path: path,
			}
			err = json.Unmarshal(data, &l)
			if err != nil {
				return err
			}
			if l.Mode == "" {
				l.Mode = l.ID
			}
			languages = append(languages, l)
		}
		return filepath.SkipDir
	})
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Name < languages[j].Name
	})
	return err
}

func loadSample(file, path string) (string, error) {
	content, err := ioutil.ReadFile(filepath.Join(path, file))
	if err != nil {
		return "", err
	}
	return string(content), nil
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
	s, err := runCode(req)
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
		if l.ID == language {
			return l, nil
		}
	}
	return lang{}, fmt.Errorf("invalid language '%s'", language)
}

func runCode(req request) (string, error) {
	lang, err := findLanguage(req.Lang)
	if err != nil {
		return "", err
	}
	dir, err := ioutil.TempDir("/tmp/dtc", "dtc-"+req.Lang+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	err = ioutil.WriteFile(filepath.Join(dir, lang.File), []byte(req.Code), 0666)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("docker", "run", "--rm",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", dir+":/dtc", "dtc-"+req.Lang)
	output := bytes.Buffer{}
	cmd.Stderr = &output
	cmd.Stdout = &output
	cmd.Run()
	return output.String(), nil
}

func sampleHandler(w http.ResponseWriter, r *http.Request) {
	l, err := findLanguage(r.FormValue("lang"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	content, err := loadSample(r.FormValue("file"), l.path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, content)
}

func languagesHandler(w http.ResponseWriter, r *http.Request) {
	buf, err := json.Marshal(languages)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(buf))
}
