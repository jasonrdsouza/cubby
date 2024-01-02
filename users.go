package main

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

/* todo
 * - add Caddy users
 * - client support for passing auth values in
 *    - and reader/ writer group values
 *    - and writeup in readme
 * - make favicon "admin" writable only
 * - pentesting
 * - backup and then reupload existing prod caddy stuff
 * - readme explanation how it works
 *    - adding/ removing users (and why it's a seperate codepath)
 *    - default choices (world readable - user writable, why introduce groups with only 3?)
 *    - backup/ restore strategy for upgrading
 * - major version release (mention new data bucket to avoid data loss)
 */

const (
	USERS_BUCKET    = "users"
	BCRYPT_COST     = 8
	USERNAME_HEADER = "username"
	TOKEN_HEADER    = "token"
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

type User interface {
	Name() string
	PasswordMatches(string) bool
	InGroup(Group) bool
}

type RegularUser struct {
	Username     string
	PasswordHash []byte
	Groups       []Group
}

func NewUser(name string, password string, groups []Group) *RegularUser {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), BCRYPT_COST)
	if err != nil {
		panic(err)
	}

	return &RegularUser{
		Username:     name,
		PasswordHash: passwordHash,
		Groups:       groups,
	}
}

func (u *RegularUser) Name() string { return u.Username }

func (u *RegularUser) PasswordMatches(password string) bool {
	err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password))
	return err == nil
}

func (u *RegularUser) InGroup(group Group) bool {
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

type AnonymousUser struct{}

func (u *AnonymousUser) Name() string                         { return "Anonymous" }
func (u *AnonymousUser) PasswordMatches(password string) bool { return false }
func (u *AnonymousUser) InGroup(group Group) bool             { return group == PublicGroup }
