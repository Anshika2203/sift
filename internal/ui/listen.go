package ui

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// remoteEvent carries actions submitted over the --listen HTTP server into the
// main event loop.
type remoteEvent struct {
	t    time.Time
	acts []action
}

func (e *remoteEvent) When() time.Time { return e.t }

// startListener starts an HTTP server that accepts an action list in the request
// body (e.g. "reload(ls)+down" or "change-query(foo)") and injects it into the
// running finder. A bare port binds to 127.0.0.1 for safety — the actions can
// run shell commands, so only expose this to trusted local callers.
func startListener(addr string, screen tcell.Screen) error {
	if !strings.Contains(addr, ":") {
		addr = "127.0.0.1:" + addr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		acts, perr := parseActionList(strings.TrimSpace(string(body)))
		if perr != nil {
			http.Error(w, perr.Error(), http.StatusBadRequest)
			return
		}
		screen.PostEvent(&remoteEvent{t: time.Now(), acts: acts})
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	return nil
}
