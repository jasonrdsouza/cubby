package main

import "time"

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

func (m *CubbyMetadata) MarkUpdated() {
	m.UpdatedAt = time.Now()
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
