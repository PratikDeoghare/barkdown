// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func serve(opts Options, addr string, stdout, stderr io.Writer) error {
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	opts.Serving = host
	if opts.Out == "" {
		dir, err := os.MkdirTemp("", "barkdown-serve-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		opts.Out = dir
	}
	var mu sync.RWMutex
	reload := newHub()
	var site *Site
	build := func() {
		mu.Lock()
		defer mu.Unlock()
		start := time.Now()
		s, err := Build(opts)
		if err != nil {
			fmt.Fprintf(stderr, "build failed: %v\n", err)
			return
		}
		site = s
		report(stdout, stderr, s, time.Since(start))
		reload.send("reload")
	}
	build()
	if site == nil {
		return fmt.Errorf("the first build failed; fix the error above and run barkdown serve again")
	}
	base := site.basePath
	go watch(opts.Src, opts.Out, build)

	mux := http.NewServeMux()
	mux.Handle(base+"__livereload", reload)
	files := http.StripPrefix(strings.TrimSuffix(base, "/"), notFound(opts.Out, base))
	mux.Handle(base, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	if base != "/" {
		mux.Handle("/", http.RedirectHandler(base, http.StatusFound))
	}
	fmt.Fprintf(stdout, "serving at http://%s%s (Ctrl-C to stop)\n", host, base)
	return http.ListenAndServe(addr, mux)
}

func notFound(dir, base string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+r.URL.Path)))
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			p = filepath.Join(p, "index.html")
		}
		if _, err := os.Stat(p); err != nil {
			if page, err := os.ReadFile(filepath.Join(dir, "404.html")); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				w.Write(page)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func watch(src, out string, rebuild func()) {
	absOut, _ := filepath.Abs(out)
	last := fingerprint(src, absOut)
	for range time.Tick(500 * time.Millisecond) {
		if now := fingerprint(src, absOut); now != last {
			last = now
			rebuild()
		}
	}
}

func fingerprint(src, absOut string) uint64 {
	h := fnv.New64a()
	filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			abs, _ := filepath.Abs(p)
			if abs == absOut || (p != src && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", p, info.Size(), info.ModTime().UnixNano())
		}
		return nil
	})
	return h.Sum64()
}

type hub struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

func newHub() *hub { return &hub{clients: map[chan string]bool{}} }

func (h *hub) send(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- msg:
		default:
		}
	}
}

func (h *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	c := make(chan string, 1)
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-c:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
