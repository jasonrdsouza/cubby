package main

import (
	"fmt"
	"regexp"
	"slices"

	"golang.org/x/crypto/bcrypt"
)

const (
	USERS_BUCKET        = "users"
	BCRYPT_COST         = 8
	CUBBY_READER_HEADER = "X-Cubby-Reader"
)

// Credential names double as Basic Auth usernames and as entries in the
// comma-separated X-Cubby-Reader header, so they are kept to a safe charset.
var credentialNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// ValidateCredentialName reports whether name can be used for a new admin or
// reader key. "public" and "private" are reserved X-Cubby-Reader values.
func ValidateCredentialName(name string) error {
	if !credentialNamePattern.MatchString(name) {
		return fmt.Errorf("invalid name %q: use 1-64 letters, digits, '.', '_' or '-'", name)
	}
	if name == "public" || name == "private" {
		return fmt.Errorf("invalid name %q: reserved", name)
	}
	return nil
}

type User interface {
	Name() string
	IsAdmin() bool
	CanRead(*CubbyMetadata) bool
	String() string
}

// RegularUser is a stored credential: either an admin, who can read and write
// everything, or a reader key, which can only read the blobs that list it.
type RegularUser struct {
	Username     string
	PasswordHash []byte
	Admin        bool // the zero value (reader key) is the least privileged
}

func NewUser(name string, password string, admin bool) RegularUser {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), BCRYPT_COST)
	if err != nil {
		panic(err)
	}

	return RegularUser{
		Username:     name,
		PasswordHash: passwordHash,
		Admin:        admin,
	}
}

func (u RegularUser) Name() string  { return u.Username }
func (u RegularUser) IsAdmin() bool { return u.Admin }

func (u RegularUser) PasswordMatches(password string) bool {
	err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password))
	return err == nil
}

func (u RegularUser) CanRead(m *CubbyMetadata) bool {
	return u.Admin || m.Public || slices.Contains(m.ReaderKeys, u.Username)
}

func (u RegularUser) Role() string {
	if u.Admin {
		return "admin"
	}
	return "reader key"
}

func (u RegularUser) String() string {
	return fmt.Sprintf("User{Username: %s, Role: %s}", u.Username, u.Role())
}

type AnonymousUser struct{}

func (u AnonymousUser) Name() string                  { return "Anonymous" }
func (u AnonymousUser) IsAdmin() bool                 { return false }
func (u AnonymousUser) CanRead(m *CubbyMetadata) bool { return m.Public }
func (u AnonymousUser) String() string                { return "AnonymousUser{}" }
