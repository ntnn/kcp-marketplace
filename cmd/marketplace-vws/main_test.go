package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/ntnn/kcp-marketplace/pkg/datasource"
)

// fakeChecker allows workspaces/exports whose cluster is in allow, and can be
// made to error.
type fakeChecker struct {
	allow map[string]bool
	err   error
}

func (f fakeChecker) CanAccessWorkspace(_ context.Context, _ user.Info, cluster string) (bool, error) {
	return f.allow[cluster], f.err
}

func (f fakeChecker) CanBindAPIExport(_ context.Context, _ user.Info, cluster, _ string) (bool, error) {
	return f.allow[cluster], f.err
}

func TestFilterWorkspacesKeepsAllowedInOrder(t *testing.T) {
	ws := []datasource.Workspace{
		{Path: "root:a", Cluster: "a"},
		{Path: "root:b", Cluster: "b"},
		{Path: "root:c", Cluster: "c"},
	}
	checker := fakeChecker{allow: map[string]bool{"a": true, "c": true}}

	got, err := filterWorkspaces(context.Background(), checker, &user.DefaultInfo{Name: "u"}, ws)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Cluster)
	assert.Equal(t, "c", got[1].Cluster)
}

func TestFilterAPIExportsKeepsAllowed(t *testing.T) {
	axs := []datasource.APIExport{
		{Path: "root", Cluster: "root", Name: "x"},
		{Path: "root", Cluster: "root", Name: "y"},
	}
	// Deny everything.
	got, err := filterAPIExports(context.Background(), fakeChecker{allow: map[string]bool{}}, &user.DefaultInfo{Name: "u"}, axs)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFilterWorkspacesPropagatesError(t *testing.T) {
	ws := []datasource.Workspace{{Path: "root:a", Cluster: "a"}}
	_, err := filterWorkspaces(context.Background(), fakeChecker{err: errors.New("boom")}, &user.DefaultInfo{Name: "u"}, ws)
	require.Error(t, err)
}
