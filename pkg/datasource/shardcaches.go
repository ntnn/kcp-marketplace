package datasource

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"

	mcpcache "github.com/kcp-dev/multicluster-provider/pkg/cache"
)

// ShardCaches owns one WildcardCache per shard and registers them on the AggregateCache.
type ShardCaches struct {
	scheme *runtime.Scheme
	agg    *mcpcache.AggregateCache

	// started is closed once Start has captured the manager context
	started chan struct{}

	mu          sync.Mutex
	baseCtx     context.Context
	active      map[string]context.CancelFunc // shard id -> cache stop
	firstSynced bool
}

func newShardCaches(scheme *runtime.Scheme, agg *mcpcache.AggregateCache) *ShardCaches {
	return &ShardCaches{
		scheme:  scheme,
		agg:     agg,
		started: make(chan struct{}),
		active:  map[string]context.CancelFunc{},
	}
}

// Start captures the manager context and blocks until it is cancelled, then stops every per-shard cache. It satisfies manager.Runnable.
func (s *ShardCaches) Start(ctx context.Context) error {
	s.mu.Lock()
	s.baseCtx = ctx
	close(s.started)
	s.mu.Unlock()

	<-ctx.Done()

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cancel := range s.active {
		s.agg.RemoveCache(id)
		cancel()
		delete(s.active, id)
	}
	return nil
}

// ensure builds and syncs a WildcardCache for shard id if not already present.
func (s *ShardCaches) ensure(ctx context.Context, id string, cfg *rest.Config) error {
	select {
	case <-s.started:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.mu.Lock()
	base := s.baseCtx
	_, exists := s.active[id]
	s.mu.Unlock()
	if exists {
		return nil
	}

	wc, err := mcpcache.NewWildcardCache(cfg, ctrlcache.Options{Scheme: s.scheme})
	if err != nil {
		return fmt.Errorf("building wildcard cache for shard %q: %w", id, err)
	}
	cctx, cancel := context.WithCancel(base)
	go func() { _ = wc.Start(cctx) }()
	if !wc.WaitForCacheSync(cctx) {
		cancel()
		return fmt.Errorf("shard %q cache failed to sync", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[id]; ok {
		// Lost a race; keep the existing cache and drop this one.
		cancel()
		return nil
	}
	s.agg.AddCache(id, wc)
	s.active[id] = cancel
	s.firstSynced = true
	return nil
}

// remove stops and unregisters the cache for shard id, if present.
func (s *ShardCaches) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.active[id]; ok {
		s.agg.RemoveCache(id)
		cancel()
		delete(s.active, id)
	}
}

// Synced reports whether at least one shard cache has synced, gating readiness.
func (s *ShardCaches) Synced() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.firstSynced
}
