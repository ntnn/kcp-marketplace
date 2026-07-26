package sar

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"

	kcpfake "github.com/kcp-dev/client-go/kubernetes/fake"
	kcptesting "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/testing"
)

// sarReactor returns a fake client that answers SubjectAccessReviews with allow
// and records every created review.
func sarReactor(allow bool) (*kcpfake.ClusterClientset, *[]*authzv1.SubjectAccessReview) {
	client := kcpfake.NewClientset()
	var seen []*authzv1.SubjectAccessReview
	client.PrependReactor("create", "subjectaccessreviews",
		func(action kcptesting.Action) (bool, runtime.Object, error) {
			sar := action.(kcptesting.CreateAction).GetObject().(*authzv1.SubjectAccessReview)
			seen = append(seen, sar)
			out := sar.DeepCopy()
			out.Status.Allowed = allow
			return true, out, nil
		})
	return client, &seen
}

func TestCanAccessWorkspaceSpec(t *testing.T) {
	client, seen := sarReactor(true)
	c := New(client, time.Minute)

	u := &user.DefaultInfo{Name: "alice", Groups: []string{"team-a"}, Extra: map[string][]string{"scope": {"x"}}}
	allowed, err := c.CanAccessWorkspace(context.Background(), u, "clusterA")
	require.NoError(t, err)
	assert.True(t, allowed)

	require.Len(t, *seen, 1)
	spec := (*seen)[0].Spec
	assert.Equal(t, "alice", spec.User)
	assert.Equal(t, []string{"team-a"}, spec.Groups)
	assert.Equal(t, authzv1.ExtraValue{"x"}, spec.Extra["scope"])
	require.NotNil(t, spec.NonResourceAttributes)
	assert.Equal(t, "access", spec.NonResourceAttributes.Verb)
	assert.Equal(t, "/", spec.NonResourceAttributes.Path)
	assert.Nil(t, spec.ResourceAttributes)
}

func TestCanBindAPIExportSpec(t *testing.T) {
	client, seen := sarReactor(false)
	c := New(client, time.Minute)

	u := &user.DefaultInfo{Name: "bob"}
	allowed, err := c.CanBindAPIExport(context.Background(), u, "root", "tenancy.kcp.io")
	require.NoError(t, err)
	assert.False(t, allowed)

	require.Len(t, *seen, 1)
	ra := (*seen)[0].Spec.ResourceAttributes
	require.NotNil(t, ra)
	assert.Equal(t, "bind", ra.Verb)
	assert.Equal(t, "apis.kcp.io", ra.Group)
	assert.Equal(t, "apiexports", ra.Resource)
	assert.Equal(t, "tenancy.kcp.io", ra.Name)
}

func TestCachesDecision(t *testing.T) {
	client, seen := sarReactor(true)
	c := New(client, time.Minute)
	u := &user.DefaultInfo{Name: "alice"}

	for i := 0; i < 3; i++ {
		_, err := c.CanAccessWorkspace(context.Background(), u, "clusterA")
		require.NoError(t, err)
	}
	// Repeated identical checks hit the cache; only one SAR is issued.
	assert.Len(t, *seen, 1)

	// A different user is a distinct cache key.
	_, err := c.CanAccessWorkspace(context.Background(), &user.DefaultInfo{Name: "carol"}, "clusterA")
	require.NoError(t, err)
	assert.Len(t, *seen, 2)
}

func TestCacheTTLExpiry(t *testing.T) {
	client, seen := sarReactor(true)
	c := New(client, time.Millisecond)
	u := &user.DefaultInfo{Name: "alice"}

	_, err := c.CanAccessWorkspace(context.Background(), u, "clusterA")
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	_, err = c.CanAccessWorkspace(context.Background(), u, "clusterA")
	require.NoError(t, err)
	assert.Len(t, *seen, 2)
}
