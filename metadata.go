package main

import (
	"fmt"
	"time"
)

type CubbyMetadata struct {
	ContentType string
	UpdatedAt   time.Time
	Readers     Group
	Writers     Group
}

func (m *CubbyMetadata) String() string {
	return "CubbyMetadata{ContentType: " + m.ContentType + ", UpdatedAt: " + m.UpdatedAt.String() + ", Readers: " + m.Readers.String() + ", Writers: " + m.Writers.String() + "}"
}

func (m *CubbyMetadata) Empty() bool {
	return *m == CubbyMetadata{}
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

func (m *CubbyMetadata) UpdateReaders(group Group) {
	if group != UnknownGroup {
		m.Readers = group
	} else { // group not specified
		// use existing reader group if present
		if m.Readers != UnknownGroup {
			// do nothing
		} else {
			// default to public reads if no reader group exists or is specified
			m.Readers = PublicGroup
		}
	}
}

func (m *CubbyMetadata) UpdateWriters(group Group) {
	if group != UnknownGroup {
		m.Writers = group
	} else { // group not specified
		if m.Writers != UnknownGroup {
			// do nothing
		} else {
			// allow authenticated user writes by default
			m.Writers = UserGroup
		}
	}
}
