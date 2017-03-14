# reloadproxy
`reloadproxy` restarts your server and reloads what's in your browser, anytime changes are detected.

### reloadproxy is currently only a proof of concept.

Anytime your files are modified or any files are added or deleted, `reloadproxy` automatically restarts your server and displays your changes in real time in your browser.

# Installation

```shell
go get -u github.com/radovskyb/reloadproxy
```

# Usage

Assuming you have your server set to run at `http://localhost:9000` and you want `reloadproxy` to run it's server at `http://localhost:9001`, then simply run the command:
```shell
reloadproxy main.go
```

# Example

![reloadproxy.gif](https://github.com/radovskyb/reloadproxy/blob/master/reloadproxy.gif)
