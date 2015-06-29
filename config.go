package main

import (
	"io"
	"io/ioutil"

	"golang.org/x/crypto/ssh"

	"gopkg.in/yaml.v2"
)

type config struct {
	Store struct {
		Implementation string
		FileStore      struct {
			Directory string
		}
	}
	Clients   []configClient
	Protocols struct {
		MaxSize int64 `yaml:"max_size"`
		HTTP    struct {
			Enabled  bool
			Listener string
		} `yaml:"http,omitempty"`
		HTTPS struct {
			Enabled         bool
			Listener        string
			CertificatePath string `yaml:"certificate_path"`
			PrivateKeyPath  string `yaml:"private_key_path"`
		} `yaml:"https,omitempty"`
		SCP struct {
			Enabled  bool
			Listener string
			HostKey  string `yaml:"host_key"`
			Clients  []struct {
				Hostname  string
				PublicKey string `yaml:"public_key"`
			} `yaml:"scp,omitempty"`
		}
	}
}

type configClient struct {
	Hostname            string
	AdditionalAddresses []string
	Protocols           struct {
		HTTP struct {
			Enabled bool
		}
		HTTPS struct {
			Enabled bool
		}
		SCP struct {
			Enabled         bool
			PublicKey       string `yaml:"public_key,omitempty"`
			parsedPublicKey *ssh.PublicKey
			Password        string
		}
	}
}

func (c *config) Load(r io.Reader) error {
	confbytes, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(confbytes, c)
}
