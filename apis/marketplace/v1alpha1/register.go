package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group for marketplace resources.
const GroupName = "marketplace.kcp.io"

// SchemeGroupVersion is the group version used to register these objects.
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

var (
	// SchemeBuilder collects functions that add types to a scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme registers the marketplace types with a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource and returns a Group-qualified GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&AccessibleWorkspace{},
		&AccessibleWorkspaceList{},
		&BindableAPIExport{},
		&BindableAPIExportList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
