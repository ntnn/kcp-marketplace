package datasource

import (
	"sort"

	"github.com/kcp-dev/logicalcluster/v3"
	"k8s.io/apimachinery/pkg/runtime"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
)

// The annotation where kcp stamps the workspace path on objects.
const pathAnnotation = "kcp.io/path"

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	must(corev1alpha1.AddToScheme(scheme))
	must(apisv1alpha2.AddToScheme(scheme))
	return scheme
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func mapWorkspaces(list *corev1alpha1.LogicalClusterList) []Workspace {
	byCluster := map[string]Workspace{}
	for i := range list.Items {
		item := &list.Items[i]
		path := item.Annotations[pathAnnotation]
		cluster := logicalcluster.From(item).String()
		// Skip system/internal logical clusters without a workspace path.
		if path == "" || cluster == "" {
			continue
		}
		if item.Status.Phase != corev1alpha1.LogicalClusterPhaseReady {
			continue
		}
		byCluster[cluster] = Workspace{Path: path, Cluster: cluster}
	}

	out := make([]Workspace, 0, len(byCluster))
	for _, w := range byCluster {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func mapAPIExports(list *apisv1alpha2.APIExportList) []APIExport {
	byKey := map[string]APIExport{}
	for i := range list.Items {
		item := &list.Items[i]
		path := item.Annotations[pathAnnotation]
		cluster := logicalcluster.From(item).String()
		if path == "" || item.Name == "" {
			continue
		}
		var resources []Resource
		for _, r := range item.Spec.Resources {
			resources = append(resources, Resource{Group: r.Group, Resource: r.Name})
		}
		byKey[path+"/"+item.Name] = APIExport{
			Path:         path,
			Cluster:      cluster,
			Name:         item.Name,
			IdentityHash: item.Status.IdentityHash,
			Resources:    resources,
		}
	}

	out := make([]APIExport, 0, len(byKey))
	for _, a := range byKey {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Name < out[j].Name
	})
	return out
}
