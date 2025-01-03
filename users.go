package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	USERS_BUCKET        = "users"
	BCRYPT_COST         = 8
	CUBBY_READER_HEADER = "X-Cubby-Reader"
	CUBBY_WRITER_HEADER = "X-Cubby-Writer"
)

type Group int

// Do not rearrange these constants or else the enum values stored in the DB
// will be corrupted.
const (
	UnknownGroup Group = iota
	AdminGroup
	UserGroup
	PublicGroup
)

func StringToGroup(groupString string) Group {
	switch strings.ToLower(groupString) {
	case "admin":
		return AdminGroup
	case "user":
		return UserGroup
	case "public":
		return PublicGroup
	default:
		return UnknownGroup
	}
}

func (g Group) String() string {
	switch g {
	case AdminGroup:
		return "admin"
	case UserGroup:
		return "user"
	case PublicGroup:
		return "public"
	default:
		return "unknown"
	}
}

type User interface {
	Name() string
	PasswordMatches(string) bool
	InGroup(Group) bool
	String() string
}

type RegularUser struct {
	Username     string
	PasswordHash []byte
	Groups       []Group
}

func NewUser(name string, password string, groups []Group) RegularUser {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), BCRYPT_COST)
	if err != nil {
		panic(err)
	}

	return RegularUser{
		Username:     name,
		PasswordHash: passwordHash,
		Groups:       groups,
	}
}

func (u RegularUser) Name() string { return u.Username }

func (u RegularUser) PasswordMatches(password string) bool {
	err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password))
	return err == nil
}

func (u RegularUser) InGroup(group Group) bool {
	for _, userGroup := range u.Groups {
		if userGroup == group {
			return true
		} else if userGroup == AdminGroup {
			// Let Admins have access to everything
			return true
		}
	}
	return false
}

func (u RegularUser) String() string {
	return fmt.Sprintf("User{Username: %s, Groups: %v}", u.Username, u.Groups)
}

type AnonymousUser struct{}

func (u AnonymousUser) Name() string                         { return "Anonymous" }
func (u AnonymousUser) PasswordMatches(password string) bool { return false }
func (u AnonymousUser) InGroup(group Group) bool             { return group == PublicGroup }
func (u AnonymousUser) String() string                       { return "AnonymousUser{}" }
