package apiserver

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// ListFunc synthesizes the list of objects visible to the given user.
type ListFunc func(ctx context.Context, u user.Info, opts *metainternalversion.ListOptions) (runtime.Object, error)

// StorageConfig configures a read-only REST storage.
type StorageConfig struct {
	// Singular is the singular resource name (e.g. "accessibleworkspace").
	Singular string
	// Kind is the object kind (e.g. "AccessibleWorkspace").
	Kind string
	// NewFunc returns a new empty object.
	NewFunc func() runtime.Object
	// NewListFunc returns a new empty list object.
	NewListFunc func() runtime.Object
	// List synthesizes the list for the requesting user.
	List ListFunc
	// Columns are the table columns for `kubectl get`.
	Columns []metav1.TableColumnDefinition
	// Row extracts a table row's cells from a single object, in Columns order.
	Row func(obj runtime.Object) []interface{}
}

// storage is an etcd-free, read-only, cluster-scoped REST storage that
// synthesizes list results per request from an injected ListFunc.
type storage struct {
	cfg StorageConfig
}

var (
	_ rest.Storage              = &storage{}
	_ rest.Scoper               = &storage{}
	_ rest.KindProvider         = &storage{}
	_ rest.SingularNameProvider = &storage{}
	_ rest.Lister               = &storage{}
	_ rest.TableConvertor       = &storage{}
)

// NewStorage builds a read-only REST storage from cfg.
//
// The genericapiserver requires one rest.Storage and rest.Lister per resource.
// This thin implementation satisfies genericapiserver and allows to serve the ephemeral resources.
func NewStorage(cfg StorageConfig) rest.Storage {
	return &storage{cfg: cfg}
}

// New implements rest.Storage.
func (s *storage) New() runtime.Object { return s.cfg.NewFunc() }

// Destroy implements rest.Storage.
func (s *storage) Destroy() {}

// NamespaceScoped implements rest.Scoper.
func (s *storage) NamespaceScoped() bool { return false }

// Kind implements rest.KindProvider.
func (s *storage) Kind() string { return s.cfg.Kind }

// GetSingularName implements rest.SingularNameProvider.
func (s *storage) GetSingularName() string { return s.cfg.Singular }

// NewList implements rest.Lister.
func (s *storage) NewList() runtime.Object { return s.cfg.NewListFunc() }

// List implements rest.Lister.
func (s *storage) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	u, _ := apirequest.UserFrom(ctx)
	return s.cfg.List(ctx, u, opts)
}

// ConvertToTable implements rest.TableConvertor.
func (s *storage) ConvertToTable(_ context.Context, object runtime.Object, _ runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{ColumnDefinitions: s.cfg.Columns}

	if meta.IsListType(object) {
		if l, err := meta.ListAccessor(object); err == nil {
			table.ResourceVersion = l.GetResourceVersion()
			table.Continue = l.GetContinue()
		}
		if err := meta.EachListItem(object, func(obj runtime.Object) error {
			table.Rows = append(table.Rows, s.row(obj))
			return nil
		}); err != nil {
			return nil, err
		}
		return table, nil
	}

	if m, err := meta.Accessor(object); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
	}
	table.Rows = append(table.Rows, s.row(object))
	return table, nil
}

func (s *storage) row(obj runtime.Object) metav1.TableRow {
	return metav1.TableRow{
		Cells:  s.cfg.Row(obj),
		Object: runtime.RawExtension{Object: obj},
	}
}
