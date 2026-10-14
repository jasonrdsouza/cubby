package main

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"slices"

	"github.com/boltdb/bolt"
)

func (c *CubbyServer) FetchUser(name string, password string) User {
	if name == "" || password == "" {
		return &AnonymousUser{}
	}

	var user *RegularUser
	c.db.View(func(tx *bolt.Tx) error {
		user = c.getUser(name, tx)
		return nil
	})
	if user == nil {
		c.log.Printf("Unable to find user with name: %s", name)
		return &AnonymousUser{}
	}

	if user.PasswordMatches(password) {
		c.log.Printf("Found valid user: %s", user)
		return user
	} else {
		c.log.Printf("Invalid credentials specified for user with name: %s", name)
		return &AnonymousUser{}
	}
}

// getUser returns the stored user with the given name, or nil if there is none.
func (c *CubbyServer) getUser(name string, tx *bolt.Tx) *RegularUser {
	value := tx.Bucket([]byte(c.usersBucket)).Get([]byte(name))
	if value == nil {
		return nil
	}
	var user RegularUser
	if err := gob.NewDecoder(bytes.NewReader(value)).Decode(&user); err != nil {
		c.log.Printf("Error decoding user %s: %v", name, err)
		return nil
	}
	return &user
}

func (c *CubbyServer) putUser(user RegularUser, tx *bolt.Tx) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(user); err != nil {
		return err
	}
	return tx.Bucket([]byte(c.usersBucket)).Put([]byte(user.Name()), buf.Bytes())
}

func (c *CubbyServer) isReaderKey(name string, tx *bolt.Tx) bool {
	user := c.getUser(name, tx)
	return user != nil && !user.Admin
}

func (c *CubbyServer) ListUsers() []RegularUser {
	var users []RegularUser
	c.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(c.usersBucket)).ForEach(func(k, _ []byte) error {
			if user := c.getUser(string(k), tx); user != nil {
				users = append(users, *user)
			}
			return nil
		})
	})
	return users
}

// AddAdmin creates an admin, or resets the password of an existing one.
func (c *CubbyServer) AddAdmin(name string, password string) error {
	if err := ValidateCredentialName(name); err != nil {
		return err
	}
	if password == "" {
		return errors.New("admin password must not be empty")
	}
	err := c.db.Update(func(tx *bolt.Tx) error {
		if existing := c.getUser(name, tx); existing != nil && !existing.Admin {
			return fmt.Errorf("%s is already a reader key", name)
		}
		return c.putUser(NewUser(name, password, true), tx)
	})

	if err != nil {
		c.log.Printf("Error adding admin %s: %v", name, err)
	} else {
		c.log.Printf("Successfully added admin: %s", name)
	}
	return err
}

// AddReaderKey creates a reader key with a generated secret, which it returns.
// With regenerate, it instead replaces the secret of an existing reader key.
func (c *CubbyServer) AddReaderKey(name string, regenerate bool) (string, error) {
	if err := ValidateCredentialName(name); err != nil {
		return "", err
	}
	secret := rand.Text()
	err := c.db.Update(func(tx *bolt.Tx) error {
		existing := c.getUser(name, tx)
		switch {
		case existing != nil && existing.Admin:
			return fmt.Errorf("%s is an admin, not a reader key", name)
		case existing != nil && !regenerate:
			return fmt.Errorf("reader key %s already exists", name)
		case existing == nil && regenerate:
			return fmt.Errorf("reader key %s does not exist", name)
		}
		return c.putUser(NewUser(name, secret, false), tx)
	})

	if err != nil {
		c.log.Printf("Error adding reader key %s: %v", name, err)
		return "", err
	}
	c.log.Printf("Successfully added reader key: %s", name)
	return secret, nil
}

// RemoveUser deletes an admin or reader key. A reader key is also removed from
// every blob that lists it, so a later key of the same name gains no access.
func (c *CubbyServer) RemoveUser(name string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		if c.getUser(name, tx) == nil {
			return fmt.Errorf("no user named %s", name)
		}
		if err := tx.Bucket([]byte(c.usersBucket)).Delete([]byte(name)); err != nil {
			return err
		}
		for _, key := range c.List(tx) {
			metadata := c.GetMetadata(key, tx)
			if !slices.Contains(metadata.ReaderKeys, name) {
				continue
			}
			metadata.ReaderKeys = slices.DeleteFunc(metadata.ReaderKeys, func(k string) bool { return k == name })
			if err := c.PutMetadata(key, metadata, tx); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.log.Printf("Error removing user %s: %v", name, err)
	} else {
		c.log.Printf("Successfully removed user: %s", name)
	}
	return err
}
