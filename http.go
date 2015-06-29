package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/nrolans/configstore"
)

type ServerConfig struct {
	Store         configstore.Store
	FileSizeLimit int64
}

type HTTPServer struct {
	listener   net.Listener
	httpServer *http.Server
	wgHandlers sync.WaitGroup
	config     *ServerConfig
}

func NewHTTPServer(config *ServerConfig, listener *net.TCPListener) *HTTPServer {
	h := new(HTTPServer)
	h.listener = listener
	h.config = config
	return h
}

func (h *HTTPServer) Serve() {
	h.httpServer = &http.Server{
		Handler: h,
	}
	h.httpServer.Serve(h.listener)
}

func (h *HTTPServer) Stop() (err error) {
	log.Println("Closing the HTTP listener")
	err = h.listener.Close()
	if err != nil {
		return
	}

	log.Println("Waiting for active HTTP connections to terminate")
	h.wgHandlers.Wait()
	log.Println("HTTP Server stopped")
	return

}

func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.wgHandlers.Add(1)
	defer h.wgHandlers.Done()

	log.Printf("New HTTP request from %s\n", req.RemoteAddr)

	// Method check
	if req.Method != "PUT" {
		log.Printf("Received a %s method from %s, rejecting\n", req.Method, req.RemoteAddr)
		http.Error(w, "PUT method expected", 405)
		return
	}

	if req.ContentLength > h.config.FileSizeLimit {
		log.Printf("Received a file larger than allowed from %s (%d > %d)\n", req.RemoteAddr, req.ContentLength, h.config.FileSizeLimit)
		http.Error(w, "File too large", 403)
		return
	}

	// Hostname check
	name, err := lookupIP(req.RemoteAddr)
	if err != nil {
		log.Printf("Unable to lookup host %s\n", req.RemoteAddr)
		http.Error(w, "Host not known", 403)
		return
	}

	e := configstore.NewEntry()
	e.Name = name
	e.Date = time.Now()
	written, err := io.Copy(e, req.Body)
	if err != nil {
		log.Println("Failed to copy body into entry: %s", err)
		http.Error(w, "Failed to read body", 500)
	}
	req.Body.Close()

	if err := h.config.Store.Add(*e); err != nil {
		log.Println("Failed to write entry: %s", err)
		http.Error(w, "Failed to write entry", 500)
		return
	}

	log.Printf("Successfully received a file from %s (%s) - %d bytes\n", req.RemoteAddr, name, written)

	w.WriteHeader(200)
	fmt.Fprintf(w, "Saved %d bytes", written)

	return
}
