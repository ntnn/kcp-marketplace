// Package datasource provides cross-shard, in-process views of the kcp objects the marketplace exposes: Workspaces and APIExports.
package datasource

import "context"

// Workspace is an accessible workspace, realized by a LogicalCluster.
type Workspace struct {
	// Path is the human-readable workspace path (kcp.io/path), e.g. root:team-a.
	Path string
	// Cluster is the logical cluster name (kcp.io/cluster).
	Cluster string
}

// Resource is a single resource offered by an APIExport.
type Resource struct {
	Group    string
	Resource string
}

// PermissionClaim is a permission an APIExport requests from binding workspaces.
type PermissionClaim struct {
	Group        string
	Resource     string
	Verbs        []string
	IdentityHash string
}

// APIExport is a bindable APIExport.
type APIExport struct {
	// Path is the workspace path hosting the APIExport.
	Path string
	// Cluster is the logical cluster name hosting the APIExport.
	Cluster string
	// Name is the APIExport name.
	Name string
	// IdentityHash is the APIExport identity hash, when published.
	IdentityHash string
	// Resources are the resources the APIExport serves.
	Resources []Resource
	// PermissionClaims are the permission claims the APIExport requests.
	PermissionClaims []PermissionClaim
}

// Interface is the read side consumed by the REST storage. Returned slices are
// the full, unfiltered set; per-user SAR filtering happens above this layer.
type Interface interface {
	Workspaces(ctx context.Context) ([]Workspace, error)
	APIExports(ctx context.Context) ([]APIExport, error)
}

// Static is a fixed datasource, useful for tests and local development.
type Static struct {
	Ws []Workspace
	Ax []APIExport
}

var _ Interface = Static{}

// Workspaces implements Interface.
func (s Static) Workspaces(context.Context) ([]Workspace, error) { return s.Ws, nil }

// APIExports implements Interface.
func (s Static) APIExports(context.Context) ([]APIExport, error) { return s.Ax, nil }
