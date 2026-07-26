package datasource

import (
	"context"
	"fmt"
	"net/url"

	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// shardReconciler watches Shard objects in the kcp root workspaces and passes the events to the ShardCaches.
type shardReconciler struct {
	client     client.Client
	caches     *ShardCaches
	base       *rest.Config
	gatewayURL string
}

func (r *shardReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	shard := &corev1alpha1.Shard{}
	if err := r.client.Get(ctx, req.NamespacedName, shard); err != nil {
		if apierrors.IsNotFound(err) {
			r.caches.remove(req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	cfg, err := shardConfig(r.base, r.gatewayURL, shard)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := r.caches.ensure(ctx, shard.Name, cfg); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func shardConfig(base *rest.Config, gatewayURL string, shard *corev1alpha1.Shard) (*rest.Config, error) {
	baseURL := shard.Spec.BaseURL
	if baseURL == "" {
		return nil, fmt.Errorf("shard %q has no spec.baseURL", shard.Name)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing shard %q baseURL %q: %w", shard.Name, baseURL, err)
	}
	cfg := rest.CopyConfig(base)
	cfg.Host = gatewayURL
	cfg.ServerName = u.Hostname()
	return cfg, nil
}
