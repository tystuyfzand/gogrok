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

// DenyAll is a HostValidator to deny all custom requests
func DenyAll(host string) bool {
	return false
}
