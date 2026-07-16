// Package domain holds the value types that flow between the service and
// repository layers of the container service. They are intentionally simple
// data holders (Anemic model); business rules live in service/, and CRD
// manifest shapes live inside repository/knative.
package domain

// ProjectScope identifies the tenant + project a request operates on.
type ProjectScope struct {
	UserID    string
	ProjectID string
}

// EnvVar is a name/value pair carried through Deploy and read back on List.
type EnvVar struct {
	Name  string
	Value string
}

// DeployRequest captures the user-facing shape of a service deployment
// (independent of the Knative Service CRD schema).
type DeployRequest struct {
	Name          string
	Image         string
	Port          int32
	MinScale      int32
	MaxScale      int32
	StartupScript string
	Env           []EnvVar
}

// DeployedService is the repository's view of a running service. The
// service layer enriches this with domain-mapping status before returning
// it to callers.
type DeployedService struct {
	Name          string
	Image         string
	URL           string
	CustomDomain  string
	ResourceName  string
	Ready         bool
	Reason        string
	CreatedAt     string
	UpdatedAt     string
	Namespace     string
	ProjectID     string
	Generation    int64
	Port          int32
	MinScale      int32
	MaxScale      int32
	StartupScript string
	Env           []EnvVar
}
