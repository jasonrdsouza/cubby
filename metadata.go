package main

import "time"

type CubbyMetadata struct {
	ContentType string
	UpdatedAt   time.Time
}

func (m *CubbyMetadata) Empty() bool {
	return *m == CubbyMetadata{}
}
