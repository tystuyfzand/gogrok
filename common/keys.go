package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"io/ioutil"
)

func LoadOrGenerateKey(fs afero.Fs, file, passphrase string) (*rsa.PrivateKey, error) {
	if file == "" {
		return GenRSA(4096)
	}

	f, err := fs.Open(file)

	if err != nil {
		key, err := GenRSA(4096)

		if err != nil {
			return nil, err
		}

		f, err := fs.Create(file)

		if err != nil {
			return nil, err
		}

		err = pem.Encode(f, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})

		if err != nil {
			return nil, err
		}

		return key, err
	}

	defer f.Close()

	priv, err := ioutil.ReadAll(f)

	if err != nil {
		return nil, err
	}

	privPem, _ := pem.Decode(priv)

	var privPemBytes []byte

	if passphrase != "" {
		privPemBytes, err = x509.DecryptPEMBlock(privPem, []byte(passphrase))

		if err != nil {
			return nil, err
		}
	} else {
		privPemBytes = privPem.Bytes
	}

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPemBytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPemBytes); err != nil { // note this returns type `interface{}`
			return nil, err
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)

	// TODO: Support other key types than rsa.PrivateKey
	if !ok {
		return nil, errors.New("key isn't of type rsa.PrivateKey")
	}

	return privateKey, nil
}

// GenRSA returns a new RSA key of bits length
func GenRSA(bits int) (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)

	if err != nil {
		log.WithError(err).Fatal("Unable to generate key")
	}

	return key, err
}
