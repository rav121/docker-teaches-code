package main

import (
	"encoding/base64"
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
	fmt.Println("Parsing envs")
	if err := parseEnvs(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Starting backend server on port 8080")
	http.Handle("/", http.FileServer(http.Dir("front")))
	http.HandleFunc("/run/", runHandler)
	http.HandleFunc("/data/", dataHandler)
	http.HandleFunc("/envs/", envsHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type sample struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Input string `json:"input"`
}

type env struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	File    string   `json:"file"`
	Samples []sample `json:"samples"`
	path    string
}

var envs = []env{}

func parseEnvs() error {
	root := "envs"
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
			l := env{
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
			envs = append(envs, l)
		}
		return filepath.SkipDir
	})
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].Name < envs[j].Name
	})
	return err
}

type request struct {
	Env   string
	Code  string
	Input string
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

func findEnv(ID string) (env, error) {
	for _, l := range envs {
		if l.ID == ID {
			return l, nil
		}
	}
	return env{}, fmt.Errorf("invalid env '%s'", ID)
}

func runCode(req request, conn *websocket.Conn) error {
	env, err := findEnv(req.Env)
	if err != nil {
		return err
	}
	dir, err := ioutil.TempDir("/tmp/dtc", "dtc-"+req.Env+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	err = ioutil.WriteFile(filepath.Join(dir, env.File), []byte(req.Code), 0666)
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "run", "--rm", "-i",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", dir+":/dtc", "dtc-"+req.Env)
	outp, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errp, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if req.Input != "" {
		inp, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		b, err := base64.StdEncoding.DecodeString(req.Input)
		if err != nil {
			return err
		}
		_, err = inp.Write(b)
		if err != nil {
			fmt.Println(err)
		}
		inp.Close()
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
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	wg.Wait()
	close(send)
	return err
}

func stream(send chan<- []byte, r io.Reader) error {
	for {
		buf := make([]byte, 1024)
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
			_, err = fmt.Fprint(w, base64.StdEncoding.EncodeToString(buf))
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	l, err := findEnv(r.FormValue("env"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	file := filepath.Join(l.path, r.FormValue("file"))
	content, err := ioutil.ReadFile(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, base64.StdEncoding.EncodeToString(content))
}

func envsHandler(w http.ResponseWriter, r *http.Request) {
	buf, err := json.Marshal(envs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(buf))
}
