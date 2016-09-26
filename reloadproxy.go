package reloadproxy

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const rpTemplateSrc = `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Reload Proxy</title>
	<script src="https://code.jquery.com/jquery-3.1.1.min.js"></script> 
	<script>
		$(document).ready(function() {
			var ws = new WebSocket("ws://localhost:9000/reloadproxy/ws/{{.Path}}");
			ws.onmessage = function(e) {
				$("body").html(e.data);
			};
		});
	</script>
</head>
<body>
</body>
</html>`

var upgrader = websocket.Upgrader{}
var rpTemplate = template.Must(template.New("rpTemplate").Parse(rpTemplateSrc))
var watcher = make(chan struct{})

func init() {
	http.HandleFunc("/reloadproxy/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Path string
		}{
			strings.TrimPrefix(r.URL.Path, "/reloadproxy/"),
		}
		if err := rpTemplate.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	var cwd string
	http.HandleFunc("/reloadproxy/ws/", func(w http.ResponseWriter, r *http.Request) {
		// Get the working directory in the handler instead of on during init,
		// so if the folder get's changed or a different server is started on
		// the same host under a differnt folder instead, it is tracked.
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ReloadProxyHandler(w, r)
	})

	go startWatching(watcher, cwd)
}

func ReloadProxyHandler(w http.ResponseWriter, r *http.Request) {
	var scheme string
	if r.TLS != nil {
		scheme = "https://"
	} else {
		scheme = "http://"
	}

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
		for {
			select {
			case <-watcher:
				path := strings.TrimPrefix(r.URL.Path, "/reloadproxy/ws")
				err := conn.WriteMessage(websocket.TextMessage, reloadPage(scheme+r.Host+path))
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

func startWatching(watcher chan struct{}, dir string) {
	for {
		watcher <- struct{}{}
		time.Sleep(time.Second)
	}
}

func reloadPage(path string) []byte {
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
