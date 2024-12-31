package rexec

import (
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func signerFromPem(pemBytes []byte) (ssh.Signer, error) {

	// read pem block
	err := errors.New("cannot decode pem block, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	// NOTE handle key encrypted with password here if needed, x509.DecryptPEMBlock is obsolete

	// if x509.IsEncryptedPEMBlock(pemBlock) {
	// 	// decrypt PEM
	// 	pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
	// 	if err != nil {
	// 		return nil, fmt.Errorf("cannot decrypt PEM block %s", err.Error())
	// 	}

	// 	// get RSA, EC or DSA key
	// 	key, err := parsePemBlock(pemBlock)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	// generate signer instance from key
	// 	signer, err := ssh.NewSignerFromKey(key)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("cannot create signer from encrypted key %s", err.Error())
	// 	}

	// 	return signer, nil
	// }

	// generate signer instance from plain key
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parsie plain private key %s", err.Error())
	}

	return signer, nil
}

// func parsePemBlock(block *pem.Block) (any, error) {
// 	switch block.Type {
// 	case "RSA PRIVATE KEY":
// 		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
// 		if err != nil {
// 			return nil, fmt.Errorf("cannot parse PKCS private key %s", err.Error())
// 		} else {
// 			return key, nil
// 		}
// 	case "EC PRIVATE KEY":
// 		key, err := x509.ParseECPrivateKey(block.Bytes)
// 		if err != nil {
// 			return nil, fmt.Errorf("cannot parse EC private key %s", err.Error())
// 		} else {
// 			return key, nil
// 		}
// 	case "DSA PRIVATE KEY":
// 		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
// 		if err != nil {
// 			return nil, fmt.Errorf("cannot parse DSA private key %s", err.Error())
// 		} else {
// 			return key, nil
// 		}
// 	default:
// 		return nil, fmt.Errorf("cannot parse private key, unsupported key type %s", block.Type)
// 	}
// }

func NewSshClientConfig(user string, privateKeyOrPath string) (*ssh.ClientConfig, error) {
	reBegin := regexp.MustCompile(`-----BEGIN [ a-zA-Z0-9]+ KEY-----`)
	reEnd := regexp.MustCompile(`-----END [ a-zA-Z0-9]+ KEY-----`)
	strBegin := reBegin.FindString(privateKeyOrPath)
	strEnd := reEnd.FindString(privateKeyOrPath)
	var signer ssh.Signer
	if strBegin != "" && strEnd != "" {
		// This is an embedded private key. The only reason we support this scenario is because we want less hassle in the SAAS
		// (so we can just provide the key in the variable instead of managing a key file)
		pemWithoutCrlf := strings.NewReplacer("\n", "", "\r", "").Replace(privateKeyOrPath)
		pemWithTwoCrlfs := strings.ReplaceAll(strings.ReplaceAll(pemWithoutCrlf, strBegin, strBegin+"\n"), strEnd, "\n"+strEnd)
		var err error
		signer, err = signerFromPem([]byte(pemWithTwoCrlfs))
		if err != nil {
			return nil, fmt.Errorf("cannot use private key starting with %s: %s", strBegin, err.Error())
		}
	} else {
		keyPath := privateKeyOrPath
		if strings.HasPrefix(keyPath, "~/") {
			homeDir, _ := os.UserHomeDir()
			keyPath = filepath.Join(homeDir, keyPath[2:])
		}
		pemBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read private key file %s: %s", keyPath, err.Error())
		}
		signer, err = signerFromPem(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("cannot use private key from file %s(%s): %s", privateKeyOrPath, keyPath, err.Error())
		}
	}

	return &ssh.ClientConfig{
		Timeout: time.Duration(10 * time.Second),
		User:    user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// use known_hosts file if you care about host validation
			return nil
		},
	}, nil
}
