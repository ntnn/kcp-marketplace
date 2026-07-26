package datasource

import (
	"context"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"

	mcpcache "github.com/kcp-dev/multicluster-provider/pkg/cache"
)

type Reader struct {
	agg *mcpcache.AggregateCache
}

var _ Interface = (*Reader)(nil)

// Workspaces lists all Ready LogicalClusters across all shards.
func (r *Reader) Workspaces(ctx context.Context) ([]Workspace, error) {
	list := &corev1alpha1.LogicalClusterList{}
	if err := r.agg.List(ctx, list); err != nil {
		return nil, err
	}
	return mapWorkspaces(list), nil
}

// APIExports lists APIExports across all shards.
func (r *Reader) APIExports(ctx context.Context) ([]APIExport, error) {
	list := &apisv1alpha2.APIExportList{}
	if err := r.agg.List(ctx, list); err != nil {
		return nil, err
	}
	return mapAPIExports(list), nil
}
