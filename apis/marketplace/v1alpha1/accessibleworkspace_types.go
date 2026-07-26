package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleWorkspace is a Ready workspace the requesting user is allowed to access.
// This is an ephemeral resource like a SAR.
type AccessibleWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AccessibleWorkspaceSpec `json:"spec,omitempty"`
}

// AccessibleWorkspaceSpec describes an accessible workspace.
type AccessibleWorkspaceSpec struct {
	// Path is the human-readable workspace path (e.g. root:team:app).
	Path string `json:"path,omitempty"`
	// Cluster is the logical cluster name backing the workspace.
	Cluster string `json:"cluster,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleWorkspaceList is a list of AccessibleWorkspace.
type AccessibleWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AccessibleWorkspace `json:"items"`
}
