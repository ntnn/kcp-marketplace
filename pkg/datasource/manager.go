package datasource

import (
	"context"

	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	mcpcache "github.com/kcp-dev/multicluster-provider/pkg/cache"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
)

type Manager struct {
	mgr    mcmanager.Manager
	caches *ShardCaches
	reader *Reader
}

func NewManager(rootCfg, base *rest.Config, gatewayURL string) (*Manager, error) {
	scheme := newScheme()
	agg := mcpcache.NewAggregateCache(scheme)
	caches := newShardCaches(scheme, agg)

	mgr, err := mcmanager.New(rootCfg, nil, ctrlmanager.Options{
		Scheme:                 scheme,
		LeaderElection:         false,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return nil, err
	}

	local := mgr.GetLocalManager()
	if err := local.Add(caches); err != nil {
		return nil, err
	}

	if err := builder.ControllerManagedBy(local).
		For(&corev1alpha1.Shard{}).
		Complete(&shardReconciler{
			client:     local.GetClient(),
			caches:     caches,
			base:       base,
			gatewayURL: gatewayURL,
		}); err != nil {
		return nil, err
	}

	return &Manager{mgr: mgr, caches: caches, reader: &Reader{agg: agg}}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	return m.mgr.Start(ctx)
}

func (m *Manager) Reader() Interface {
	return m.reader
}

func (m *Manager) Synced() bool {
	return m.caches.Synced()
}
