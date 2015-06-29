package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/nrolans/configstore"
	"github.com/nrolans/configstore/file"
	"github.com/nrolans/service"
	"github.com/nrolans/service/scp"
	"golang.org/x/crypto/ssh"
)

func main() {

	// Arg parsing
	cfgfilepath := flag.String("config", "config.yaml", "Config file")
	flag.Parse()

	// Load config
	cfgfile, err := os.Open(*cfgfilepath)
	if err != nil {
		log.Fatalf("Failed to open config file: %s", err)
	}
	config := &config{}
	err = config.Load(cfgfile)
	if err != nil {
		log.Fatalf("Failed to load config file: %s", err)
	}

	// Turn KBytes to Bytes
	config.Protocols.MaxSize *= 1024

	// Prepare the configstore
	var store configstore.Store
	if config.Store.Implementation == "filestore" {
		store = file.NewFileStore(config.Store.FileStore.Directory, file.DefaultDateFormat)
	} else {
		log.Fatal("Unknown store implementation")
	}
	log.Printf("Store initialised: %s", store)

	// Setup authenticator
	adb := newAuthDB()
	for _, c := range config.Clients {
		adb.Add(&c)
	}

	// -----------
	// SCP server
	// -----------
	var scpService service.TCPServicer
	if config.Protocols.SCP.Enabled {

		// New SCP handler
		scpCopyHandler := NewSCPHandler(store)

		// SCP server config
		scpConfig := &scp.SCPConfig{
			ServerConfig: &ssh.ServerConfig{
				PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
					return adb.AuthSCPPassword(c, pass)
				},
				PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
					return adb.AuthSCPPublicKey(c, key)
				},
			},
		}

		// Load host key
		private, err := ssh.ParsePrivateKey([]byte(config.Protocols.SCP.HostKey))
		if err != nil {
			log.Fatalf("Unable to load SSH private key: %s", err)
		}
		scpConfig.AddHostKey(private)

		scpService = scp.NewSCPService(scpConfig, scpCopyHandler, nil)
		scpAddr, err := net.ResolveTCPAddr("tcp", config.Protocols.SCP.Listener)
		if err != nil {
			log.Printf("Failed to parse listener address")
		}

		scpListener, err := net.ListenTCP("tcp", scpAddr)
		if err != nil {
			log.Printf("Failed to start listener")
		}
		go scpService.Serve(*scpListener)
	}

	// -----------
	// HTTP server
	// -----------
	var httpServer *HTTPServer
	if config.Protocols.HTTP.Enabled {

		httpConfig := &ServerConfig{
			Store:         store,
			FileSizeLimit: config.Protocols.MaxSize,
		}
		httpListener, err := net.Listen("tcp", config.Protocols.HTTP.Listener)
		if err != nil {
			log.Printf("Failed to start http listener: %s", err)
		}
		httpTCPListener, _ := httpListener.(*net.TCPListener)
		httpServer = NewHTTPServer(httpConfig, httpTCPListener)
		go httpServer.Serve()
	}

	// ------------
	// HTTPS server
	// ------------
	if config.Protocols.HTTPS.Enabled {
		// TODO
	}

	// Set signal handler -- shutdown sequence
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	func() {
		sig := <-signalChan // Wait for a signal

		log.Printf("Received signal (%s)", sig)

		if config.Protocols.SCP.Enabled {
			log.Printf("Stopping SCP server")
			scpService.Stop()
		}

		if config.Protocols.HTTP.Enabled {
			log.Printf("Stopping HTTP server")
			httpServer.Stop()
		}
	}()

}

func lookupIP(addr string) (string, error) {

	// Hostname lookup to identify the switch
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	names, err := net.LookupAddr(host)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("Unable to lookup IP %s: no results\n", host)
	}
	name := names[0]
	//name = name[:len(name)-1]

	return name, nil
}
