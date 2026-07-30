package datasource

import (
	"testing"

	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func shard(name, baseURL string) *corev1alpha1.Shard {
	return &corev1alpha1.Shard{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1alpha1.ShardSpec{BaseURL: baseURL},
	}
}

func TestShardConfig(t *testing.T) {
	base := &rest.Config{Host: "https://fp.example.com:6443", BearerToken: "secret"}

	for _, tc := range []struct {
		name           string
		gatewayURL     string
		baseURL        string
		wantHost       string
		wantServerName string
		wantErr        bool
	}{{
		// Shards on separate clusters resolve on their own, so the shard is
		// dialed directly and TLS verification uses the dialed hostname.
		name:     "without a gateway the shard baseURL is dialed directly",
		baseURL:  "https://shard-sa.example.com:31443",
		wantHost: "https://shard-sa.example.com:31443",
	}, {
		// One address fronts every shard, so the shard hostname has to travel as
		// SNI for the passthrough router to pick the right backend.
		name:           "with a gateway the shard hostname becomes SNI",
		gatewayURL:     "https://gateway.svc:8443",
		baseURL:        "https://shard-sa.example.com:31443",
		wantHost:       "https://gateway.svc:8443",
		wantServerName: "shard-sa.example.com",
	}, {
		name:    "a shard without a baseURL is an error",
		baseURL: "",
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := shardConfig(base, tc.gatewayURL, shard("sa", tc.baseURL))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Host != tc.wantHost {
				t.Errorf("host: got %q, want %q", cfg.Host, tc.wantHost)
			}
			if cfg.ServerName != tc.wantServerName {
				t.Errorf("server name: got %q, want %q", cfg.ServerName, tc.wantServerName)
			}
			if cfg.BearerToken != base.BearerToken {
				t.Errorf("credentials were not carried over from the base config")
			}
			if base.Host != "https://fp.example.com:6443" {
				t.Errorf("the base config was mutated: %q", base.Host)
			}
		})
	}
}
