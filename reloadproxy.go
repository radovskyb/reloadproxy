package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	rpTemplate = template.Must(template.New("rpTemplate").Parse(rpTemplateSrc))
	upgrader   = websocket.Upgrader{}
	watcher    = make(chan struct{})

	serverFile, socketAddr, serverAddr, address, dir string

	cmd *exec.Cmd
)

func main() {
	flag.StringVar(&serverFile, "file", "", "the location of the Go server file")
	flag.StringVar(&dir, "dir", "", "the directory to watch for changes (default is the server's directory)")
	flag.StringVar(&address, "addr", "http://localhost:9001",
		"the address to run reloadproxy")
	flag.StringVar(&serverAddr, "server", "http://localhost:9000",
		"the address where the server is set to run")
	flag.Parse()

	if serverFile == "" {
		fmt.Println("Enter the location of the Go server file. (e.g. main.go)\n")
		flag.PrintDefaults()
		fmt.Println()
		os.Exit(1)
	}

	if dir == "" {
		dir = filepath.Dir(serverFile)
	}

	// Seed the random generator.
	rand.Seed(time.Now().UnixNano())

	// Generate a random string for the WebSocket url location to avoid conflicts
	// with the proxied server paths.
	alphaRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 12)
	for i := range b {
		b[i] = alphaRunes[rand.Intn(len(alphaRunes))]
	}
	socketAddr = string(b)

	listenAddr := strings.SplitAfter(address, "://")[1]

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			SocketAddr string
			Address    string
			Path       string
		}{
			SocketAddr: socketAddr,
			Address:    listenAddr,
			Path:       strings.TrimPrefix(r.URL.Path, "/"),
		}
		if err := rpTemplate.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/"+socketAddr+"/", ReloadProxyHandler)

	// Start reloadproxy's server
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Fatal(http.ListenAndServe(listenAddr, nil))
	}()

	// Start the regular server.
	startServer()

	c := make(chan os.Signal)
	signal.Notify(c, os.Kill, os.Interrupt)
	go func() {
		<-c
		killServer()
		os.Exit(1)
	}()

	// Start reloadproxy's watcher.
	done := make(chan struct{})
	go startWatching(watcher, cmd, done)
	<-done

	wg.Wait()
}

func startWatching(watcher chan struct{}, cmd *exec.Cmd, done chan struct{}) {
	first := true

	// Show the page for the first time.
	watcher <- struct{}{}

	files := []os.FileInfo{}

	for {
		newfiles := []os.FileInfo{}
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			newfiles = append(newfiles, info)
			return nil
		}); err != nil {
			log.Fatalln(err)
		}

		if len(files) != len(newfiles) {
			files = newfiles
			if !first {
				// Restart the server.
				restartServer()
			} else {
				first = false
			}
			watcher <- struct{}{}
		} else {
			for i, newfile := range newfiles {
				if newfile.ModTime() != files[i].ModTime() {
					// Restart the server.
					restartServer()
					files = newfiles
					watcher <- struct{}{}
				}
			}
		}

		time.Sleep(time.Second * 1)
	}

	close(done)
}

func ReloadProxyHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var c = make(chan struct{})
	go func(conn *websocket.Conn) {
		for {
			mType, _, err := conn.ReadMessage()
			if err != nil || mType == websocket.CloseMessage {
				c <- struct{}{}
				conn.Close()
				return
			}
		}
	}(conn)

	go func(conn *websocket.Conn) {
		path := strings.TrimPrefix(r.URL.Path, "/"+socketAddr)

		// Write the page the first time it's visited.
		err := conn.WriteMessage(websocket.TextMessage, getPage(serverAddr+path))
		if err != nil {
			conn.Close()
			return
		}

		for {
			select {
			case <-watcher:
				err := conn.WriteMessage(websocket.TextMessage, getPage(serverAddr+path))
				if err != nil {
					conn.Close()
					return
				}
			case <-c:
				return
			}
		}
	}(conn)
}

func getPage(path string) []byte {
GET:
	res, err := http.Get(path)
	if err != nil {
		if _, ok := err.(*net.AddrError); ok {
			return []byte(err.Error())
		}
		// If it's not an address error, sleep for a bit then try again.
		time.Sleep(500 * time.Millisecond)
		goto GET
	}
	defer res.Body.Close()
	slurp, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []byte(err.Error())
	}
	return slurp
}

func startServer() {
	cmd = exec.Command("go", "run", serverFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalln(err)
	}
}

func killServer() {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		log.Fatalln(err)
	}
}

func restartServer() {
	killServer()
	startServer()
}

const rpTemplateSrc = `<!DOCTYPE html><html><head>
<meta charset="UTF-8"> <title>Reload Proxy</title>
<script src="https://code.jquery.com/jquery-3.1.1.min.js"></script><script>
$(document).ready(function() {
	var ws = new WebSocket("ws://{{.Address}}/{{.SocketAddr}}/{{.Path}}");
	ws.onmessage = function(e) {
		$("body").html(e.data);
	};
});
</script></head><body></body></html>`
