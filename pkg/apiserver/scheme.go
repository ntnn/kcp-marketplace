package apiserver

import (
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	marketplacev1alpha1 "github.com/ntnn/kcp-marketplace/apis/marketplace/v1alpha1"
)

// Scheme is the marketplace scheme shared by the apiservers.
var Scheme = runtime.NewScheme()

// Codecs provides the serializers for the marketplace scheme.
var Codecs = serializer.NewCodecFactory(Scheme)

// ParameterCodec decodes request query parameters (e.g. list options).
var ParameterCodec = runtime.NewParameterCodec(Scheme)

func init() {
	utilruntime(marketplacev1alpha1.AddToScheme(Scheme))

	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
	)
	// List/watch option conversions between v1 and internal.
	utilruntime(metainternalversion.AddToScheme(Scheme))
}

func utilruntime(err error) {
	if err != nil {
		panic(err)
	}
}
