package main

import (
	"bytes"
	"encoding/gob"

	"github.com/boltdb/bolt"
)

func (c *CubbyServer) FetchUser(name string, password string) User {
	if name == "" || password == "" {
		return &AnonymousUser{}
	}

	var value []byte
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.usersBucket))
		value = b.Get([]byte(name))
		return nil
	})

	decoder := gob.NewDecoder(bytes.NewBuffer(value))
	var user RegularUser
	err := decoder.Decode(&user)
	if err != nil {
		c.log.Printf("Unable to find user with name: %s. %v", name, err)
		return &AnonymousUser{}
	}

	if user.PasswordMatches(password) {
		c.log.Printf("Found valid user: %s", user)
		return &user
	} else {
		c.log.Printf("Invalid credentials specified for user with name: %s", name)
		return &AnonymousUser{}
	}
}

func (c *CubbyServer) ListUsers() []string {
	var users []string
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.usersBucket))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			users = append(users, string(k))
		}
		return nil
	})
	return users
}

func (c *CubbyServer) AddUser(name string, password string, isAdmin bool) error {
	groups := []Group{PublicGroup, UserGroup}
	if isAdmin {
		groups = append(groups, AdminGroup)
	}
	user := NewUser(name, password, groups)

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(user)
	if err != nil {
		c.log.Printf("Error encoding new user: %s", name)
		return err
	}

	err = c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.usersBucket))
		return b.Put([]byte(user.Name()), buf.Bytes())
	})

	if err != nil {
		c.log.Printf("Error adding user: %s", name)
	} else {
		c.log.Printf("Successfully added user: %s", name)
	}
	return err
}

func (c *CubbyServer) RemoveUser(name string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.usersBucket))
		return b.Delete([]byte(name))
	})

	if err != nil {
		c.log.Printf("Error removing user: %s", name)
	} else {
		c.log.Printf("Successfully removed user: %s", name)
	}
	return err
}
