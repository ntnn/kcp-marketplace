package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BindableAPIExport is an APIExport the requesting user is allowed to bind.
// This is an ephemeral resource like a SAR.
type BindableAPIExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BindableAPIExportSpec `json:"spec,omitempty"`
}

// BindableAPIExportSpec describes a bindable APIExport.
type BindableAPIExportSpec struct {
	// Path is the workspace path hosting the APIExport.
	Path string `json:"path,omitempty"`
	// Cluster is the logical cluster name hosting the APIExport.
	Cluster string `json:"cluster,omitempty"`
	// ExportName is the APIExport name.
	ExportName string `json:"exportName,omitempty"`
	// IdentityHash is the APIExport identity hash, when published.
	IdentityHash string `json:"identityHash,omitempty"`
	// Resources are the resources exported by the APIExport.
	Resources []BindableResource `json:"resources,omitempty"`
	// PermissionClaims are the permission claims the APIExport requests.
	PermissionClaims []BindablePermissionClaim `json:"permissionClaims,omitempty"`
}

// BindableResource is a single resource offered by a BindableAPIExport.
type BindableResource struct {
	Group    string `json:"group,omitempty"`
	Resource string `json:"resource,omitempty"`
}

// BindablePermissionClaim is a permission claim requested by a BindableAPIExport.
type BindablePermissionClaim struct {
	Group    string `json:"group,omitempty"`
	Resource string `json:"resource,omitempty"`
	// Verbs are the API verbs the claim covers.
	Verbs []string `json:"verbs,omitempty"`
	// IdentityHash is the APIExport identity the claimed schema belongs to; empty for core types.
	IdentityHash string `json:"identityHash,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BindableAPIExportList is a list of BindableAPIExport.
type BindableAPIExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []BindableAPIExport `json:"items"`
}
