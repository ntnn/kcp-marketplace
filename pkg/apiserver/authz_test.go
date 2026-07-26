package apiserver

import (
	"context"
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

func TestAuthenticatedOnly(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		want   authorizer.Decision
	}{
		{"authenticated", []string{user.AllAuthenticated}, authorizer.DecisionAllow},
		{"anonymous", []string{user.AllUnauthenticated}, authorizer.DecisionNoOpinion},
		{"no groups", nil, authorizer.DecisionNoOpinion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attrs := authorizer.AttributesRecord{User: &user.DefaultInfo{Name: "u", Groups: tc.groups}}
			got, _, err := authenticatedOnly{}.Authorize(context.Background(), attrs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewAuthorizerHealthPaths(t *testing.T) {
	authz, err := newAuthorizer()
	if err != nil {
		t.Fatalf("newAuthorizer: %v", err)
	}
	anon := &user.DefaultInfo{Name: user.Anonymous, Groups: []string{user.AllUnauthenticated}}

	// Anonymous may reach health paths.
	for _, p := range []string{"/healthz", "/livez", "/readyz"} {
		d, _, err := authz.Authorize(context.Background(),
			authorizer.AttributesRecord{User: anon, Path: p, ResourceRequest: false})
		if err != nil {
			t.Fatalf("authorize %s: %v", p, err)
		}
		if d != authorizer.DecisionAllow {
			t.Fatalf("path %s: got %v, want Allow", p, d)
		}
	}

	// Anonymous is denied the API surface.
	d, _, err := authz.Authorize(context.Background(),
		authorizer.AttributesRecord{User: anon, Path: "/apis", ResourceRequest: false})
	if err != nil {
		t.Fatalf("authorize /apis: %v", err)
	}
	if d == authorizer.DecisionAllow {
		t.Fatalf("/apis for anonymous: got Allow, want deny/no-opinion")
	}
}
