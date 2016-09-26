package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/radovskyb/process"
)

var (
	rpTemplate = template.Must(template.New("rpTemplate").Parse(rpTemplateSrc))
	upgrader   = websocket.Upgrader{}
	watcher    = make(chan struct{})

	socketAddr string
	address    = "http://localhost:9001"
	serverAddr = "http://localhost:9000"
	dir        = "/Users/Benjamin/Workspace/go/src/github.com/radovskyb/reloadproxy/testserver"

	restartServer bool
	serverProcess *process.Process
)

func main() {
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

	go startWatching(watcher)

	log.Fatal(http.ListenAndServe(listenAddr, nil))
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

func startWatching(watcher chan struct{}) {
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
			watcher <- struct{}{}
		} else {
			for i, newfile := range newfiles {
				if newfile.ModTime() != files[i].ModTime() {
					files = newfiles
					watcher <- struct{}{}
				}
			}
		}

		time.Sleep(time.Second * 1)
	}
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
