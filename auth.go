package main

import (
	"bytes"
	"fmt"
	"log"

	"golang.org/x/crypto/ssh"
)

type authDB struct {
	clients map[string]*configClient
}

func newAuthDB() *authDB {
	a := &authDB{}
	a.clients = make(map[string]*configClient)
	return a
}

func (a *authDB) Add(c *configClient) {
	// Parse SSH key
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(c.Protocols.SCP.PublicKey))
	if err != nil {
		log.Fatalf("Failed to parse ssh public key for %s: %s", c.Hostname, err)
	}
	c.Protocols.SCP.parsedPublicKey = &k

	// Add to map
	a.clients[c.Hostname] = c
	for _, addr := range c.AdditionalAddresses {
		a.clients[addr] = c
	}
}

func (a authDB) findClient(address string) *configClient {
	// Hostname lookup
	hostname, err := lookupIP(address)
	if err != nil {
		log.Printf("Failed to lookup hostname: %s", err)
	} else {
		if c, ok := a.clients[hostname]; ok {
			return c
		}
	}

	// Address lookup
	if c, ok := a.clients[address]; ok {
		return c
	}

	// Client not found
	return nil
}

// SCP Password authentication check
func (a authDB) AuthSCPPassword(cmd ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {

	// Get the client
	c := a.findClient(cmd.RemoteAddr().String())
	if c == nil {
		return nil, fmt.Errorf("Unknown host %s", cmd.RemoteAddr().String())
	}

	// Check Password
	if string(pass) == c.Protocols.SCP.Password {
		return nil, nil
	}
	return nil, fmt.Errorf("Authencation failed")
}

// SCP Public Key authentication check
func (a authDB) AuthSCPPublicKey(cmd ssh.ConnMetadata, recvKey ssh.PublicKey) (*ssh.Permissions, error) {

	// Get the client
	c := a.findClient(cmd.RemoteAddr().String())
	if c == nil {
		return nil, fmt.Errorf("Unknown host %s", cmd.RemoteAddr().String())
	}

	// Verify there is a key configured
	goodKey := c.Protocols.SCP.parsedPublicKey
	if goodKey == nil {
		return nil, fmt.Errorf("Auth failed")
	}

	// Compare keys
	if bytes.Compare(recvKey.Marshal(), (*goodKey).Marshal()) == 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("Auth failed")

}
