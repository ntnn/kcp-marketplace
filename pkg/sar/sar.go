// Package sar issues per-user SubjectAccessReviews against the workspace or APIExport logical clusters.
package sar

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	kcpkubernetesclientset "github.com/kcp-dev/client-go/kubernetes"
)

// Interface is the check surface consumed by the REST list closures.
type Interface interface {
	// CanAccessWorkspace reports whether u may enter the workspace in the given logical cluster (NonResource verb=access, path=/).
	CanAccessWorkspace(ctx context.Context, u user.Info, cluster string) (bool, error)
	// CanBindAPIExport reports whether u may bind the named APIExport in the given logical cluster (resource verb=bind on apiexports/<name>).
	CanBindAPIExport(ctx context.Context, u user.Info, cluster, name string) (bool, error)
}

// Checker issues SubjectAccessReviews through a cluster-aware kube client.
// The client targets the front-proxy origin; the logical cluster is selected per call via Cluster(path), and the front-proxy routes to the hosting shard.
type Checker struct {
	client kcpkubernetesclientset.ClusterInterface
	ttl    time.Duration

	mu    sync.Mutex
	cache map[string]entry
}

type entry struct {
	allowed bool
	expires time.Time
}

var _ Interface = (*Checker)(nil)

// New returns a Checker using client, caching decisions for ttl.
func New(client kcpkubernetesclientset.ClusterInterface, ttl time.Duration) *Checker {
	return &Checker{client: client, ttl: ttl, cache: map[string]entry{}}
}

// CanAccessWorkspace implements Interface.
func (c *Checker) CanAccessWorkspace(ctx context.Context, u user.Info, cluster string) (bool, error) {
	return c.review(ctx, u, cluster, authzv1.SubjectAccessReviewSpec{
		NonResourceAttributes: &authzv1.NonResourceAttributes{Verb: "access", Path: "/"},
	})
}

// CanBindAPIExport implements Interface.
func (c *Checker) CanBindAPIExport(ctx context.Context, u user.Info, cluster, name string) (bool, error) {
	return c.review(ctx, u, cluster, authzv1.SubjectAccessReviewSpec{
		ResourceAttributes: &authzv1.ResourceAttributes{
			Verb:     "bind",
			Group:    "apis.kcp.io",
			Resource: "apiexports",
			Name:     name,
		},
	})
}

func (c *Checker) review(ctx context.Context, u user.Info, cluster string, spec authzv1.SubjectAccessReviewSpec) (bool, error) {
	spec.User = u.GetName()
	spec.UID = u.GetUID()
	spec.Groups = u.GetGroups()
	if extra := u.GetExtra(); len(extra) > 0 {
		spec.Extra = map[string]authzv1.ExtraValue{}
		for k, v := range extra {
			spec.Extra[k] = authzv1.ExtraValue(v)
		}
	}

	key := cacheKey(cluster, spec)
	if allowed, ok := c.get(key); ok {
		return allowed, nil
	}

	res, err := c.client.Cluster(logicalcluster.NewPath(cluster)).
		AuthorizationV1().SubjectAccessReviews().
		Create(ctx, &authzv1.SubjectAccessReview{Spec: spec}, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("subjectaccessreview in cluster %q: %w", cluster, err)
	}
	c.put(key, res.Status.Allowed)
	return res.Status.Allowed, nil
}

func (c *Checker) get(key string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok || time.Now().After(e.expires) {
		return false, false
	}
	return e.allowed, true
}

func (c *Checker) put(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = entry{allowed: allowed, expires: time.Now().Add(c.ttl)}
}

// cacheKey derives a stable key from the cluster and the full SAR spec (which
// already carries the user identity), so distinct users never share a decision.
func cacheKey(cluster string, spec authzv1.SubjectAccessReviewSpec) string {
	b, _ := json.Marshal(spec)
	sum := sha256.Sum256(b)
	return cluster + "|" + fmt.Sprintf("%x", sum)
}
