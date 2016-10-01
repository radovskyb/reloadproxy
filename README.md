# reloadproxy

## Needs re-writing.

`reloadproxy` restarts your server and reloads what's in your browser, anytime changes are made.

`reloadproxy` is a simple Go application that's a reverse proxy to your Go server. It also does cool stuffs. (See gif below)

Anytime your files are updated or any files are added or deleted, `reloadproxy` automatically
restarts your server and displays your changes in real time in your browser.

# Installation:

1. go get github.com/radovskyb/reloadproxy

# Usage: 

Assuming you have your server set to run at http://localhost:9000 and you want `reloadproxy`
to run at http://localhost:9001, then simply run the command:
```
reloadproxy main.go
```

# Example:

![reloadproxy.gif](https://github.com/radovskyb/reloadproxy/blob/master/reloadproxy.gif)

`reloadproxy` is just a hacked up prototype at the moment so don't expect too much :)

So far it's only been tested on my MacBook Pro so if it works for you too then awesome.
