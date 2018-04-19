package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Println(err)
		return
	}
	req := request{}
	err = json.Unmarshal(data, &req)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = runCode(req, conn)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func findLanguage(language string) (lang, error) {
	for _, l := range languages {
		if l.ID == language {
			return l, nil
		}
	}
	return lang{}, fmt.Errorf("invalid language '%s'", language)
}

func runCode(req request, conn *websocket.Conn) error {
	lang, err := findLanguage(req.Lang)
	if err != nil {
		return err
	}
	dir, err := ioutil.TempDir("/tmp/dtc", "dtc-"+req.Lang+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	err = ioutil.WriteFile(filepath.Join(dir, lang.File), []byte(req.Code), 0666)
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "run", "--rm",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", dir+":/dtc", "dtc-"+req.Lang)
	outp, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errp, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	send := make(chan []byte)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := stream(send, outp); err != nil {
			fmt.Println(err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := stream(send, errp); err != nil {
			fmt.Println(err)
		}
	}()
	go flush(send, conn)
	err = cmd.Wait()
	wg.Wait()
	close(send)
	return err
}

func stream(send chan<- []byte, r io.Reader) error {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		send <- buf[:n]
	}
}

func flush(send <-chan []byte, conn *websocket.Conn) {
	for {
		select {
		case buf, ok := <-send:
			if !ok {
				return
			}
			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer w.Close()
			_, err = w.Write(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
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
