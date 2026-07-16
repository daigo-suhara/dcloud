// Package domain holds value types shared between the compute service
// and repository layers. Intentionally anemic: business rules live in
// service/, KubeVirt CRD manifest shapes live in repository/kubevirt.
package domain

type ProjectScope struct {
	UserID    string
	ProjectID string
}

type CreateRequest struct {
	Name   string
	Image  string
	CPU    string
	Memory string
}

type MachineRecord struct {
	Name       string
	Image      string
	CPU        string
	Memory     string
	Ready      bool
	Status     string
	Reason     string
	CreatedAt  string
	UpdatedAt  string
	Namespace  string
	ProjectID  string
	Generation int64
}
