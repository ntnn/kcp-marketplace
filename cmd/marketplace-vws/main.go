// Command marketplace-vws serves the marketplace.kcp.io virtual workspace.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	kcpkubernetesclientset "github.com/kcp-dev/client-go/kubernetes"

	marketplacev1alpha1 "github.com/ntnn/kcp-marketplace/apis/marketplace/v1alpha1"
	"github.com/ntnn/kcp-marketplace/pkg/apiserver"
	"github.com/ntnn/kcp-marketplace/pkg/datasource"
	"github.com/ntnn/kcp-marketplace/pkg/sar"
)

// sarConcurrency bounds the per-list SubjectAccessReview fan-out.
const sarConcurrency = 16

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	o := apiserver.NewOptions()
	o.PathPrefix = "/services/marketplace"
	var (
		shardKubeconfig string
		gatewayURL      string
	)
	fs := pflag.NewFlagSet("marketplace-vws", pflag.ExitOnError)
	o.AddFlags(fs)
	fs.StringVar(&shardKubeconfig, "shard-kubeconfig", "",
		"Kubeconfig with a privileged identity able to list Shards and wildcard-list across each shard's /clusters/*. "+
			"If empty, a static placeholder datasource is used.")
	fs.StringVar(&gatewayURL, "gateway-url", "",
		"In-cluster address of the TLS-passthrough gateway used to reach shards, e.g. "+
			"https://eg-nodeport.envoy-gateway-system.svc.cluster.local:8443. "+
			"If empty, each shard is dialed at its own spec.baseURL, which is required "+
			"when shards are spread over several clusters.")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if err := o.Complete(); err != nil {
		return err
	}
	if err := o.Validate(); err != nil {
		return err
	}

	ctx := genericapiserver.SetupSignalContext()

	ctrl.SetLogger(klog.Background())

	mgr, ds, checker, err := buildDatasource(shardKubeconfig, gatewayURL)
	if err != nil {
		return err
	}

	server, err := apiserver.New("marketplace-vws", o, storages(ds, checker))
	if err != nil {
		return err
	}

	if mgr != nil {
		// Run the shard manager as a post-start hook so its lifetime is bound to
		// the server, and gate readiness on the first shard cache syncing.
		if err := server.AddPostStartHook("shard-manager", func(hookCtx genericapiserver.PostStartHookContext) error {
			go func() {
				if err := mgr.Start(hookCtx); err != nil {
					klog.FromContext(hookCtx).Error(err, "shard manager stopped")
				}
			}()
			return nil
		}); err != nil {
			return err
		}
		if err := server.AddReadyzChecks(healthz.NamedCheck("shards-synced", func(*http.Request) error {
			if mgr.Synced() {
				return nil
			}
			return fmt.Errorf("shard caches not synced")
		})); err != nil {
			return err
		}
	}

	return server.PrepareRun().RunWithContext(ctx)
}

// buildDatasource returns the shard manager, its read side, and the per-user SAR checker when a shard kubeconfig is configured.
func buildDatasource(kubeconfig, gatewayURL string) (*datasource.Manager, datasource.Interface, sar.Interface, error) {
	if kubeconfig == "" {
		return nil, datasource.Static{
			Ws: []datasource.Workspace{
				{Path: "root", Cluster: "root"},
				{Path: "root:team-a", Cluster: "abcd1234"},
			},
			Ax: []datasource.APIExport{
				{Path: "root", Cluster: "root", Name: "tenancy.kcp.io",
					Resources: []datasource.Resource{{Group: "tenancy.kcp.io", Resource: "workspaces"}}},
			},
		}, nil, nil
	}

	// The "base" context targets the bare front-proxy origin; it carries the
	// credentials + CA used to dial the shards, the root workspace, and SARs.
	base, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: "base"},
	).ClientConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading shard kubeconfig: %w", err)
	}

	// rootCfg targets /clusters/root to watch Shards; sarCfg targets the bare
	// origin and selects the logical cluster per call via Cluster(path).
	rootCfg := restclient.CopyConfig(base)
	sarCfg := restclient.CopyConfig(base)

	if gatewayURL != "" {
		// The kubeconfig server is the external front-proxy hostname, unreachable
		// in-cluster; dial the gateway and present the front-proxy hostname as SNI so
		// the TLS-passthrough gateway routes there and the serving cert verifies.
		fpURL, err := url.Parse(base.Host)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parsing front-proxy URL %q: %w", base.Host, err)
		}
		origin := strings.TrimRight(gatewayURL, "/")
		rootCfg.Host = origin + "/clusters/root"
		rootCfg.ServerName = fpURL.Hostname()
		sarCfg.Host = origin
		sarCfg.ServerName = fpURL.Hostname()
	} else {
		// Without a gateway the kubeconfig host is reachable as it stands and each
		// shard is dialed at its own spec.baseURL, which is the only thing that
		// works when the shards live on different clusters.
		rootCfg.Host = strings.TrimRight(base.Host, "/") + "/clusters/root"
	}

	mgr, err := datasource.NewManager(rootCfg, base, gatewayURL)
	if err != nil {
		return nil, nil, nil, err
	}

	kubeCluster, err := kcpkubernetesclientset.NewForConfig(sarCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("building SAR client: %w", err)
	}

	return mgr, mgr.Reader(), sar.New(kubeCluster, 30*time.Second), nil
}

func storages(ds datasource.Interface, checker sar.Interface) map[string]rest.Storage {
	return map[string]rest.Storage{
		"accessibleworkspaces": apiserver.NewStorage(apiserver.StorageConfig{
			Singular:    "accessibleworkspace",
			Kind:        "AccessibleWorkspace",
			NewFunc:     func() runtime.Object { return &marketplacev1alpha1.AccessibleWorkspace{} },
			NewListFunc: func() runtime.Object { return &marketplacev1alpha1.AccessibleWorkspaceList{} },
			List:        listAccessibleWorkspaces(ds, checker),
			Columns: []metav1.TableColumnDefinition{
				{Name: "Name", Type: "string", Format: "name"},
				{Name: "Path", Type: "string"},
				{Name: "Cluster", Type: "string"},
			},
			Row: func(obj runtime.Object) []interface{} {
				aw, ok := obj.(*marketplacev1alpha1.AccessibleWorkspace)
				if !ok {
					return []interface{}{"", "", ""}
				}
				return []interface{}{aw.Name, aw.Spec.Path, aw.Spec.Cluster}
			},
		}),
		"bindableapiexports": apiserver.NewStorage(apiserver.StorageConfig{
			Singular:    "bindableapiexport",
			Kind:        "BindableAPIExport",
			NewFunc:     func() runtime.Object { return &marketplacev1alpha1.BindableAPIExport{} },
			NewListFunc: func() runtime.Object { return &marketplacev1alpha1.BindableAPIExportList{} },
			List:        listBindableAPIExports(ds, checker),
			Columns: []metav1.TableColumnDefinition{
				{Name: "Name", Type: "string", Format: "name"},
				{Name: "Path", Type: "string"},
				{Name: "Export", Type: "string"},
				{Name: "Cluster", Type: "string"},
			},
			Row: func(obj runtime.Object) []interface{} {
				be, ok := obj.(*marketplacev1alpha1.BindableAPIExport)
				if !ok {
					return []interface{}{"", "", "", ""}
				}
				return []interface{}{be.Name, be.Spec.Path, be.Spec.ExportName, be.Spec.Cluster}
			},
		}),
	}
}

// listAccessibleWorkspaces maps the datasource workspaces to the API type, filtered per-user by SubjectAccessReview (verb=access, path=/) when a checker is configured.
func listAccessibleWorkspaces(ds datasource.Interface, checker sar.Interface) apiserver.ListFunc {
	return func(ctx context.Context, u user.Info, _ *metainternalversion.ListOptions) (runtime.Object, error) {
		ws, err := ds.Workspaces(ctx)
		if err != nil {
			return nil, err
		}
		if checker != nil {
			ws, err = filterWorkspaces(ctx, checker, u, ws)
			if err != nil {
				return nil, err
			}
		}
		list := &marketplacev1alpha1.AccessibleWorkspaceList{}
		for _, w := range ws {
			list.Items = append(list.Items, marketplacev1alpha1.AccessibleWorkspace{
				ObjectMeta: metav1.ObjectMeta{Name: w.Cluster},
				Spec:       marketplacev1alpha1.AccessibleWorkspaceSpec{Path: w.Path, Cluster: w.Cluster},
			})
		}
		return list, nil
	}
}

// listBindableAPIExports maps the datasource exports to the API type, filtered per-user by SubjectAccessReview (verb=bind on apiexports/<name>) when a checker is configured.
func listBindableAPIExports(ds datasource.Interface, checker sar.Interface) apiserver.ListFunc {
	return func(ctx context.Context, u user.Info, _ *metainternalversion.ListOptions) (runtime.Object, error) {
		axs, err := ds.APIExports(ctx)
		if err != nil {
			return nil, err
		}
		if checker != nil {
			axs, err = filterAPIExports(ctx, checker, u, axs)
			if err != nil {
				return nil, err
			}
		}
		list := &marketplacev1alpha1.BindableAPIExportList{}
		for _, a := range axs {
			var resources []marketplacev1alpha1.BindableResource
			for _, r := range a.Resources {
				resources = append(resources, marketplacev1alpha1.BindableResource{Group: r.Group, Resource: r.Resource})
			}
			var claims []marketplacev1alpha1.BindablePermissionClaim
			for _, c := range a.PermissionClaims {
				claims = append(claims, marketplacev1alpha1.BindablePermissionClaim{
					Group: c.Group, Resource: c.Resource, Verbs: c.Verbs, IdentityHash: c.IdentityHash,
				})
			}
			list.Items = append(list.Items, marketplacev1alpha1.BindableAPIExport{
				ObjectMeta: metav1.ObjectMeta{Name: a.Name},
				Spec: marketplacev1alpha1.BindableAPIExportSpec{
					Path: a.Path, Cluster: a.Cluster, ExportName: a.Name,
					IdentityHash: a.IdentityHash, Resources: resources, PermissionClaims: claims,
				},
			})
		}
		return list, nil
	}
}

// filterWorkspaces fans out one access SAR per workspace and keeps the allowed ones, preserving order.
func filterWorkspaces(ctx context.Context, checker sar.Interface, u user.Info, ws []datasource.Workspace) ([]datasource.Workspace, error) {
	keep := make([]bool, len(ws))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(sarConcurrency)
	for i := range ws {
		g.Go(func() error {
			ok, err := checker.CanAccessWorkspace(gctx, u, ws[i].Cluster)
			keep[i] = ok
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	out := make([]datasource.Workspace, 0, len(ws))
	for i, w := range ws {
		if keep[i] {
			out = append(out, w)
		}
	}
	return out, nil
}

// filterAPIExports fans out one bind SAR per export and keeps the allowed ones, preserving order.
func filterAPIExports(ctx context.Context, checker sar.Interface, u user.Info, axs []datasource.APIExport) ([]datasource.APIExport, error) {
	keep := make([]bool, len(axs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(sarConcurrency)
	for i := range axs {
		g.Go(func() error {
			ok, err := checker.CanBindAPIExport(gctx, u, axs[i].Cluster, axs[i].Name)
			keep[i] = ok
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	out := make([]datasource.APIExport, 0, len(axs))
	for i, a := range axs {
		if keep[i] {
			out = append(out, a)
		}
	}
	return out, nil
}
