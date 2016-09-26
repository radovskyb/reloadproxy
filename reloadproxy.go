package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var rpTemplate = template.Must(template.New("rpTemplate").Parse(rpTemplateSrc))
var watcher = make(chan struct{})

var address = "http://localhost:9001"
var serverAddr = "http://localhost:9000"
var dir = "/Users/Benjamin/Workspace/go/src/github.com/radovskyb/reloadproxy/testserver"

var socketAddr string

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

	http.HandleFunc("/reloadproxy/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			SocketAddr string
			Address    string
			Path       string
		}{
			SocketAddr: socketAddr,
			Address:    listenAddr,
			Path:       strings.TrimPrefix(r.URL.Path, "/reloadproxy/"),
		}
		if err := rpTemplate.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/reloadproxy/"+socketAddr+"/", ReloadProxyHandler)

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
		path := strings.TrimPrefix(r.URL.Path, "/reloadproxy/"+socketAddr)

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

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatalln(err)
	}
	numFiles := len(files)
	for {
		time.Sleep(time.Second)

		// Check if the number of files is different.
		files, err = ioutil.ReadDir(dir)
		if err != nil {
			log.Fatalln(err)
		}
		if len(files) != numFiles {
			// Reload the page.
			watcher <- struct{}{}

			// Update numFiles to be len(files)
			numFiles = len(files)

			// Continue the file watching loop.
			continue
		}

		// 		// Run a simple filepath.Walk and check if any files have been changed
		// 		// or modified.
		// 		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// add to map so can check again after.
		// 			return nil
		// 		}); err != nil {
		// 			errc <- err
		// 		}
		// 		watcher <- struct{}{}
		// time.Sleep(time.Second)
	}
}

func getPage(path string) []byte {
	res, err := http.Get(path)
	if err != nil {
		return []byte(err.Error())
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
	var ws = new WebSocket("ws://{{.Address}}/reloadproxy/{{.SocketAddr}}/{{.Path}}");
	ws.onmessage = function(e) {
		$("body").html(e.data);
	};
});
</script></head><body></body></html>`
