//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	tenancyv1alpha1 "github.com/kcp-dev/sdk/apis/tenancy/v1alpha1"
	kcpclientset "github.com/kcp-dev/sdk/client/clientset/versioned/cluster"

	kcpkubernetesclientset "github.com/kcp-dev/client-go/kubernetes"
	"github.com/kcp-dev/multicluster-provider/envtest"

	"github.com/ntnn/kcp-marketplace/pkg/datasource"
	"github.com/ntnn/kcp-marketplace/pkg/sar"
)

const (
	userAllowed = "user-allowed"
	userDenied  = "user-denied"
	exportName  = "widgets.example.io"
)

func TestAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	env := &envtest.Sharded{WorkDir: t.TempDir()}
	require.NoError(t, env.Start(ctx), "start sharded kcp")
	t.Cleanup(func() { _ = env.Stop() })
	cfg := env.Config()

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err)
	kubeClient, err := kcpkubernetesclientset.NewForConfig(cfg)
	require.NoError(t, err)

	wsPath, cluster := createWorkspace(ctx, t, kcpClient, "market-a")
	createAPIExport(ctx, t, kcpClient, wsPath, exportName)
	grantAccess(ctx, t, kubeClient, cluster, userAllowed)
	grantBind(ctx, t, kubeClient, cluster, userAllowed, exportName)

	base := mastersConfig(t, cfg, env.WorkDir)
	rootShardURL := shardBaseURL(ctx, t, kcpClient, "root")
	rootCfg := restclient.CopyConfig(base)
	rootCfg.Host = rootShardURL + "/clusters/root"
	mgr, err := datasource.NewManager(rootCfg, base, "")
	require.NoError(t, err)
	go func() { _ = mgr.Start(ctx) }()
	require.Eventually(t, mgr.Synced, 60*time.Second, 500*time.Millisecond, "shard cache never synced")

	reader := mgr.Reader()

	t.Run("datasource sees workspace", func(t *testing.T) {
		require.Eventually(t, func() bool {
			ws, err := reader.Workspaces(ctx)
			if err != nil {
				return false
			}
			for _, w := range ws {
				if w.Path == "root:market-a" && w.Cluster == cluster {
					return true
				}
			}
			return false
		}, 30*time.Second, 500*time.Millisecond)
	})

	t.Run("datasource sees apiexport", func(t *testing.T) {
		require.Eventually(t, func() bool {
			axs, err := reader.APIExports(ctx)
			if err != nil {
				return false
			}
			for _, a := range axs {
				if a.Name == exportName && a.Cluster == cluster && a.Path == "root:market-a" {
					return true
				}
			}
			return false
		}, 30*time.Second, 500*time.Millisecond)
	})

	// SAR: filter per user against real kcp authorization.
	checker := sar.New(kubeClient, 5*time.Second)

	t.Run("access SAR follows RBAC", func(t *testing.T) {
		allowed, err := checker.CanAccessWorkspace(ctx, &user.DefaultInfo{Name: userAllowed}, cluster)
		require.NoError(t, err)
		assert.True(t, allowed, "granted user should have access")

		denied, err := checker.CanAccessWorkspace(ctx, &user.DefaultInfo{Name: userDenied}, cluster)
		require.NoError(t, err)
		assert.False(t, denied, "ungranted user should be denied access")
	})

	t.Run("bind SAR follows RBAC", func(t *testing.T) {
		allowed, err := checker.CanBindAPIExport(ctx, &user.DefaultInfo{Name: userAllowed}, cluster, exportName)
		require.NoError(t, err)
		assert.True(t, allowed, "granted user should be able to bind")

		denied, err := checker.CanBindAPIExport(ctx, &user.DefaultInfo{Name: userDenied}, cluster, exportName)
		require.NoError(t, err)
		assert.False(t, denied, "ungranted user should be denied bind")
	})

}

// createWorkspace creates a workspace under root and waits until it is Ready, returning its path and logical cluster name.
func createWorkspace(ctx context.Context, t *testing.T, client *kcpclientset.ClusterClientset, name string) (logicalcluster.Path, string) {
	t.Helper()
	root := logicalcluster.NewPath("root")
	_, err := client.Cluster(root).TenancyV1alpha1().Workspaces().Create(ctx,
		&tenancyv1alpha1.Workspace{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
	require.NoError(t, err, "create workspace")

	var cluster string
	require.Eventually(t, func() bool {
		ws, err := client.Cluster(root).TenancyV1alpha1().Workspaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		cluster = ws.Spec.Cluster
		return ws.Status.Phase == corev1alpha1.LogicalClusterPhaseReady && cluster != ""
	}, 60*time.Second, time.Second, "workspace never became ready")

	return root.Join(name), cluster
}

func createAPIExport(ctx context.Context, t *testing.T, client *kcpclientset.ClusterClientset, wsPath logicalcluster.Path, name string) {
	t.Helper()
	_, err := client.Cluster(wsPath).ApisV1alpha2().APIExports().Create(ctx,
		&apisv1alpha2.APIExport{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
	require.NoError(t, err, "create apiexport")
}

func grantAccess(ctx context.Context, t *testing.T, kube kcpkubernetesclientset.ClusterInterface, cluster, username string) {
	t.Helper()
	path := logicalcluster.NewPath(cluster)
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "marketplace-access"},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"access"}, NonResourceURLs: []string{"/"}}},
	}
	_, err := kube.Cluster(path).RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{})
	require.NoError(t, err, "create access clusterrole")
	bindRole(ctx, t, kube, path, "marketplace-access", "marketplace-access", username)
}

func grantBind(ctx context.Context, t *testing.T, kube kcpkubernetesclientset.ClusterInterface, cluster, username, export string) {
	t.Helper()
	path := logicalcluster.NewPath(cluster)
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "marketplace-bind"},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"apis.kcp.io"},
			Resources:     []string{"apiexports"},
			ResourceNames: []string{export},
			Verbs:         []string{"bind"},
		}},
	}
	_, err := kube.Cluster(path).RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{})
	require.NoError(t, err, "create bind clusterrole")
	bindRole(ctx, t, kube, path, "marketplace-bind", "marketplace-bind", username)
}

func bindRole(ctx context.Context, t *testing.T, kube kcpkubernetesclientset.ClusterInterface, path logicalcluster.Path, crbName, roleName, username string) {
	t.Helper()
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: crbName},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: roleName},
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: username}},
	}
	_, err := kube.Cluster(path).RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	require.NoError(t, err, "create clusterrolebinding")
}

func mastersConfig(t *testing.T, cfg *restclient.Config, workDir string) *restclient.Config {
	t.Helper()
	kcpDir := filepath.Join(workDir, ".kcp")
	caCertPEM, err := os.ReadFile(filepath.Join(kcpDir, "client-ca.crt"))
	require.NoError(t, err)
	caKeyPEM, err := os.ReadFile(filepath.Join(kcpDir, "client-ca.key"))
	require.NoError(t, err)

	caCert, caKey := parseCA(t, caCertPEM, caKeyPEM)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "marketplace", Organization: []string{"system:masters"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	out := restclient.CopyConfig(cfg)
	out.CertFile, out.KeyFile, out.BearerToken, out.BearerTokenFile = "", "", "", ""
	out.CertData = certPEM
	out.KeyData = keyPEM
	return out
}

func parseCA(t *testing.T, certPEM, keyPEM []byte) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block, "decode CA cert")
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	kblock, _ := pem.Decode(keyPEM)
	require.NotNil(t, kblock, "decode CA key")
	key, err := x509.ParsePKCS1PrivateKey(kblock.Bytes)
	require.NoError(t, err)
	return cert, key
}

func shardBaseURL(ctx context.Context, t *testing.T, client *kcpclientset.ClusterClientset, name string) string {
	t.Helper()
	s, err := client.Cluster(logicalcluster.NewPath("root")).CoreV1alpha1().Shards().Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err, "get shard %q", name)
	require.NotEmpty(t, s.Spec.BaseURL, "shard %q has no baseURL", name)
	return s.Spec.BaseURL
}
