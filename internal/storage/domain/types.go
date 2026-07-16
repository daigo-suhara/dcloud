// Package domain holds value types shared between the storage service
// and repository layers.
package domain

type BucketRecord struct {
	Name         string
	Endpoint     string
	Ready        bool
	Status       string
	CreatedAt    string
	ProjectID    string
	ResourceName string
}
