package main

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type CubbyMetadata struct {
	ContentType string
	UpdatedAt   time.Time
	Public      bool     // the zero value (private) is the least privileged
	ReaderKeys  []string // reader keys that may read a private blob
}

func (m *CubbyMetadata) String() string {
	return fmt.Sprintf("CubbyMetadata{ContentType: %s, UpdatedAt: %s, Access: %s}", m.ContentType, m.UpdatedAt, m.Access())
}

func (m *CubbyMetadata) Empty() bool {
	return m.ContentType == "" && m.UpdatedAt.IsZero() && !m.Public && len(m.ReaderKeys) == 0
}

// Access describes who can read the blob, in X-Cubby-Reader header syntax.
func (m *CubbyMetadata) Access() string {
	if m.Public {
		return "public"
	}
	if len(m.ReaderKeys) == 0 {
		return "private"
	}
	return strings.Join(m.ReaderKeys, ",")
}

// SetAccess makes the blob public, or private to admins plus readerKeys.
func (m *CubbyMetadata) SetAccess(public bool, readerKeys []string) {
	m.Public = public
	m.ReaderKeys = nil
	if !public && len(readerKeys) > 0 {
		m.ReaderKeys = readerKeys
	}
}

// ParseAccess parses an X-Cubby-Reader header value: "public", "private"
// (admins only), or a comma-separated list of reader key names.
func ParseAccess(header string) (public bool, readerKeys []string, err error) {
	switch v := strings.TrimSpace(header); v {
	case "public":
		return true, nil, nil
	case "private":
		return false, nil, nil
	}
	for name := range strings.SplitSeq(header, ",") {
		name = strings.TrimSpace(name)
		if err := ValidateCredentialName(name); err != nil {
			return false, nil, err
		}
		if !slices.Contains(readerKeys, name) {
			readerKeys = append(readerKeys, name)
		}
	}
	return false, readerKeys, nil
}

func (m *CubbyMetadata) SetContentType(contentType string) {
	m.ContentType = contentType
}

// MarkUpdated sets UpdatedAt to the current time, guaranteeing it moves
// strictly forward so that the derived ETag changes on every write.
func (m *CubbyMetadata) MarkUpdated() {
	now := time.Now()
	if !now.After(m.UpdatedAt) {
		now = m.UpdatedAt.Add(time.Nanosecond)
	}
	m.UpdatedAt = now
}

// ETag returns the strong entity tag for the object, derived from UpdatedAt.
// Legacy objects with a zero UpdatedAt have no ETag (empty string).
func (m *CubbyMetadata) ETag() string {
	if m.UpdatedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf(`"%d"`, m.UpdatedAt.UnixNano())
}
