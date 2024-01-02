package main

import "time"

type CubbyMetadata struct {
	ContentType string
	UpdatedAt   time.Time
	Readers     Group
	Writers     Group
}

func (m *CubbyMetadata) Empty() bool {
	return *m == CubbyMetadata{}
}
