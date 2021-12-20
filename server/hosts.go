package server

import (
    "bufio"
    "bytes"
    _ "embed"
    "math/rand"
    "time"
)

var (
    //go:embed animals.txt
    animalBytes []byte
    animals []string
)

func init() {
    animals = make([]string, 0)

    s := bufio.NewScanner(bytes.NewReader(animalBytes))

    for s.Scan() {
        animals = append(animals, s.Text())
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