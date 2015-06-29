package main

import (
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/nrolans/configstore"
	"github.com/nrolans/service/scp"
)

// SCP handlers
type SCPHandler struct {
	conn  ssh.ServerConn
	name  string
	store configstore.Store
}

func NewSCPHandler(store configstore.Store) *SCPHandler {
	s := new(SCPHandler)
	s.store = store
	return s
}

func (h *SCPHandler) SinkRequest(conn ssh.ServerConn, parameters scp.Parameter, pattern string) bool {

	h.conn = conn

	// Remote address lookup
	var err error
	h.name, err = lookupIP(conn.RemoteAddr().String())
	if err != nil {
		log.Println("Failed to lookup IP: %s", err)
		return false
	}

	log.Println("Accepting SCP request from %s", h.name)
	return true
}

func (h SCPHandler) FileRequest(mode os.FileMode, size int64, filename string) scp.Status {
	return *scp.NewStatus(scp.OK, "")
}

func (h *SCPHandler) FileCopy(r io.Reader) scp.Status {
	// Prepare a new entry
	e := configstore.NewEntry()
	e.Name = h.name
	e.Date = time.Now()

	// Copy the content
	n, err := io.Copy(e, r)
	if err != nil {
		log.Printf("Error copying file to entry: %s", err)
		return *scp.NewStatus(scp.Warning, "Error copying file")
	}
	log.Printf("Copied %d bytes", n)

	err = h.store.Add(*e)
	if err != nil {
		log.Printf("Error writing entry to store: %s", err)
		return *scp.NewStatus(scp.Warning, "Error saving file")
	}

	return *scp.NewStatus(scp.OK, "")
}

func (h *SCPHandler) DirRequest(mode os.FileMode, size int64, dirname string) scp.Status {
	return *scp.NewStatus(scp.Warning, "Not implemented")
}

func (h *SCPHandler) DirEndRequest() scp.Status {
	return *scp.NewStatus(scp.Warning, "Not implemented")
}

// SCP Authentication
type SCPAuthDB struct {
	Clients map[string]ssh.PublicKey
}

func NewSCPAuthDB() *SCPAuthDB {
	s := new(SCPAuthDB)
	s.Clients = make(map[string]ssh.PublicKey)
	return s
}

func (s *SCPAuthDB) Add(hostname string, key ssh.PublicKey) {
	s.Clients[hostname] = key
}
