package store

import (
	"encoding/json"
	"github.com/boltdb/bolt"
)

type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore creates a new boltdb backed Store instance
func NewBoltStore(path string) (Store, error) {
	db, err := bolt.Open(path, 0644, bolt.DefaultOptions)

	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("hosts"))
		return err
	})

	if err != nil {
		return nil, err
	}

	return &BoltStore{
		db: db,
	}, nil
}

// Has checks if the host exists in the hosts bucket
func (b *BoltStore) Has(host string) bool {
	var exists bool

	b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hosts"))

		if b.Get([]byte(host)) != nil {
			exists = true
		}

		return nil
	})

	return exists
}

// Get retrieves and deserializes a host from the hosts bucket
func (b *BoltStore) Get(key string) (*Host, error) {
	var host Host

	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hosts"))

		data := b.Get([]byte(key))

		if data == nil {
			return ErrNoHost
		}

		return json.Unmarshal(data, &host)
	})

	if err != nil {
		return nil, err
	}

	return &host, nil
}

// Add updates the hosts bucket and puts a json-serialized version of Host
func (b *BoltStore) Add(host Host) error {
	data, err := json.Marshal(host)

	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hosts"))

		return b.Put([]byte(host.Host), data)
	})
}

// Remove updates the hosts bucket and deletes the key
func (b *BoltStore) Remove(key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hosts"))

		return b.Delete([]byte(key))
	})
}
