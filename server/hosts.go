package server

import (
	"bufio"
	"bytes"
	_ "embed"
	"math/rand"
	"strings"
	"time"
)

var (
	//go:embed animals.txt
	animalBytes []byte
	animals     []string
)

func init() {
	animals = make([]string, 0)

	s := bufio.NewScanner(bytes.NewReader(animalBytes))

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if line[0] == '#' {
			continue

		}
		animals = append(animals, line)
	}

	rand.Seed(time.Now().UTC().UnixNano())
}

// RandomAnimal is a basic HostProvider using animal names
func RandomAnimal() string {
	return animals[rand.Intn(len(animals))]
}

// Animals returns the animal slice
func Animals() []string {
	return animals
}

// DenyAll is a HostValidator to deny all custom requests
func DenyAll(host string) bool {
	return false
}

// DenyPrefixIn denies hosts in slice s
func DenyPrefixIn(s []string) HostValidator {
	return func(host string) bool {
		idx := strings.Index(host, ".")

		if idx != -1 {
			host = host[0:idx]
		}

		for _, val := range s {
			if val == host {
				return false
			}
		}

		return true
	}
}

// SuffixIn checks hosts for a suffix value
// Note: Suffix is automatically prepended with .
func SuffixIn(s []string) HostValidator {
	return func(host string) bool {
		for _, val := range s {
			if strings.HasSuffix(host, "."+val) {
				return true
			}
		}

		return false
	}
}

// ValidateMulti checks all specified validators before denying hosts
func ValidateMulti(validators ...HostValidator) HostValidator {
	return func(host string) bool {
		for _, validator := range validators {
			if !validator(host) {
				return false
			}
		}

		return true
	}
}
