package datasource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
)

const clusterAnnotation = "kcp.io/cluster"

func lc(cluster, path string, phase corev1alpha1.LogicalClusterPhaseType) corev1alpha1.LogicalCluster {
	ann := map[string]string{}
	if cluster != "" {
		ann[clusterAnnotation] = cluster
	}
	if path != "" {
		ann[pathAnnotation] = path
	}
	return corev1alpha1.LogicalCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Annotations: ann},
		Status:     corev1alpha1.LogicalClusterStatus{Phase: phase},
	}
}

func TestMapWorkspaces(t *testing.T) {
	list := &corev1alpha1.LogicalClusterList{Items: []corev1alpha1.LogicalCluster{
		lc("rootcl", "", corev1alpha1.LogicalClusterPhaseReady),             // no path -> skipped
		lc("bcl", "root:team-b", corev1alpha1.LogicalClusterPhaseReady),     // kept
		lc("pcl", "root:new", corev1alpha1.LogicalClusterPhaseInitializing), // not ready -> skipped
		lc("acl", "root:team-a", corev1alpha1.LogicalClusterPhaseReady),     // kept
		lc("bcl", "root:team-b", corev1alpha1.LogicalClusterPhaseReady),     // duplicate -> deduped
	}}

	ws := mapWorkspaces(list)
	require.Len(t, ws, 2)
	assert.Equal(t, Workspace{Path: "root:team-a", Cluster: "acl"}, ws[0])
	assert.Equal(t, Workspace{Path: "root:team-b", Cluster: "bcl"}, ws[1])
}

func export(cluster, path, name string, resources ...[2]string) apisv1alpha2.APIExport {
	var rs []apisv1alpha2.ResourceSchema
	for _, r := range resources {
		rs = append(rs, apisv1alpha2.ResourceSchema{Group: r[0], Name: r[1]})
	}
	return apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{
			clusterAnnotation: cluster,
			pathAnnotation:    path,
		}},
		Spec:   apisv1alpha2.APIExportSpec{Resources: rs},
		Status: apisv1alpha2.APIExportStatus{IdentityHash: "hash-" + name},
	}
}

func TestMapAPIExports(t *testing.T) {
	list := &apisv1alpha2.APIExportList{Items: []apisv1alpha2.APIExport{
		export("rootcl", "root", "tenancy.kcp.io", [2]string{"tenancy.kcp.io", "workspaces"}),
		export("acl", "root:team-a", "widgets.example.io", [2]string{"example.io", "widgets"}),
		export("rootcl", "root", "tenancy.kcp.io", [2]string{"tenancy.kcp.io", "workspaces"}), // dup
	}}

	axs := mapAPIExports(list)
	require.Len(t, axs, 2)
	assert.Equal(t, "root", axs[0].Path)
	assert.Equal(t, "tenancy.kcp.io", axs[0].Name)
	assert.Equal(t, "hash-tenancy.kcp.io", axs[0].IdentityHash)
	assert.Equal(t, []Resource{{Group: "tenancy.kcp.io", Resource: "workspaces"}}, axs[0].Resources)
	assert.Equal(t, "root:team-a", axs[1].Path)
	assert.Equal(t, "widgets.example.io", axs[1].Name)
}

func TestMapAPIExportsPermissionClaims(t *testing.T) {
	e := export("acl", "root:team-a", "widgets.example.io")
	e.Spec.PermissionClaims = []apisv1alpha2.PermissionClaim{{
		GroupResource: apisv1alpha2.GroupResource{Group: "", Resource: "configmaps"},
		Verbs:         []string{"get", "list"},
		IdentityHash:  "abc",
	}}
	axs := mapAPIExports(&apisv1alpha2.APIExportList{Items: []apisv1alpha2.APIExport{e}})
	require.Len(t, axs, 1)
	assert.Equal(t, []PermissionClaim{{
		Group: "", Resource: "configmaps", Verbs: []string{"get", "list"}, IdentityHash: "abc",
	}}, axs[0].PermissionClaims)
}
