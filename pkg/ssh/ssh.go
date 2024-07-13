package ssh

import (
	"bytes"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/mitchellh/go-homedir"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func K() {
	fmt.Println("sshKeyAdd called")
	name, _ := cmd.Flags().GetString("name")
	publicKeyPath, _ := cmd.Flags().GetString("public-key-path")
	privateKeyPath, _ := cmd.Flags().GetString("private-key-path")

	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	privateKeyPath = strings.Replace(privateKeyPath, "~", home, 1)
	publicKeyPath = strings.Replace(publicKeyPath, "~", home, 1)

	var (
		data []byte
	)
	if publicKeyPath == "-" {
		data, err = ioutil.ReadAll(os.Stdin)
	} else {
		data, err = ioutil.ReadFile(publicKeyPath)
	}
	if err != nil {
		log.Fatalln(err)
	}
	publicKey := string(data)

	opts := hcloud.SSHKeyCreateOpts{
		Name:      name,
		PublicKey: publicKey,
	}

	context := AppConf.Context
	client := AppConf.Client
	sshKey, res, err := client.SSHKey.Create(context, opts)

	if res.StatusCode == http.StatusConflict {
		pkey, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			log.Fatalln(err)
		}
		// check if the key is already in to local app config
		for _, sshKey := range AppConf.Config.SSHKeys {
			localData, err := ioutil.ReadFile(sshKey.PublicKeyPath)
			if err != nil {
				log.Fatalln(err)
			}
			localPkey, _, _, _, err := ssh.ParseAuthorizedKey(localData)
			if err != nil {
				log.Fatalln(err)
			}
			// if the key is in the local app config print a message and return
			if bytes.Equal(pkey.Marshal(), localPkey.Marshal()) {
				fmt.Printf("SSH key does already exist in your config as %s\n", sshKey.Name)
				return
			}
		}
		// if the key is not in the local app config, fetch it from hetzner
		sshKeys, err := client.SSHKey.All(context)
		if err != nil {
			log.Fatalln(err)
		}
		for _, sshKeyHetzner := range sshKeys {
			hetznerPkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshKeyHetzner.PublicKey))
			if err != nil {
				log.Fatalln(err)
			}
			if bytes.Equal(pkey.Marshal(), hetznerPkey.Marshal()) {
				fmt.Printf("SSH key does already exist on hetzner as '%s'\n", sshKeyHetzner.Name)
				fmt.Printf("SSH key will be added to your config as '%s'\n", sshKeyHetzner.Name)
				// We replace the failed request response with the fetched sshkey that has the same public key
				sshKey = sshKeyHetzner
				break
			}
			if sshKeyHetzner.Name == name {
				log.Fatalf("Name '%s' is already taken!", name)
			}
		}
	} else if err != nil {
		log.Fatalln(err)
	}

	AppConf.Config.AddSSHKey(clustermanager.SSHKey{
		Name:           sshKey.Name,
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
	})

	AppConf.Config.WriteCurrentConfig()

	fmt.Printf("SSH key %s(%d) created\n", sshKey.Name, sshKey.ID)
}
