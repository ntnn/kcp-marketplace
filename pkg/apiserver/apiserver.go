// Package apiserver build the genericapiserver for the marketplace API.
package apiserver

import (
	cryptox509 "crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/util/cert"

	marketplacev1alpha1 "github.com/ntnn/kcp-marketplace/apis/marketplace/v1alpha1"
	generatedopenapi "github.com/ntnn/kcp-marketplace/pkg/generated/openapi"
)

// Options configures the marketplace apiserver.
type Options struct {
	SecureServing *genericoptions.SecureServingOptionsWithLoopback

	// RequestHeader authn: front-proxy terminates the client and
	// forwards the authenticated identity as X-Remote-* headers over
	// mTLS with a client cert signed by RequestHeaderClientCAFile.
	RequestHeaderClientCAFile        string
	RequestHeaderAllowedNames        []string
	RequestHeaderUsernameHeaders     []string
	RequestHeaderGroupHeaders        []string
	RequestHeaderExtraHeaderPrefixes []string

	// PathPrefix to strip from requests.
	PathPrefix string
}

// NewOptions returns Options with kcp-front-proxy compatible defaults.
func NewOptions() *Options {
	o := &Options{
		SecureServing:                    genericoptions.NewSecureServingOptions().WithLoopback(),
		RequestHeaderUsernameHeaders:     []string{"X-Remote-User"},
		RequestHeaderGroupHeaders:        []string{"X-Remote-Group"},
		RequestHeaderExtraHeaderPrefixes: []string{"X-Remote-Extra-"},
	}
	o.SecureServing.BindPort = 6444
	return o
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	o.SecureServing.AddFlags(fs)
	fs.StringVar(&o.RequestHeaderClientCAFile, "requestheader-client-ca-file", o.RequestHeaderClientCAFile,
		"Root CA bundle to verify client certificates of the front-proxy before trusting X-Remote-* headers.")
	fs.StringSliceVar(&o.RequestHeaderAllowedNames, "requestheader-allowed-names", o.RequestHeaderAllowedNames,
		"Allowed client certificate common names; an empty list allows any name verified by the CA.")
	fs.StringSliceVar(&o.RequestHeaderUsernameHeaders, "requestheader-username-headers", o.RequestHeaderUsernameHeaders,
		"Request headers to inspect for the username.")
	fs.StringSliceVar(&o.RequestHeaderGroupHeaders, "requestheader-group-headers", o.RequestHeaderGroupHeaders,
		"Request headers to inspect for groups.")
	fs.StringSliceVar(&o.RequestHeaderExtraHeaderPrefixes, "requestheader-extra-headers-prefix", o.RequestHeaderExtraHeaderPrefixes,
		"Request header prefixes to inspect for extra user info.")
	fs.StringVar(&o.PathPrefix, "path-prefix", o.PathPrefix,
		"Path prefix the front-proxy maps to this server (e.g. /services/marketplace-access); stripped before routing.")
}

func (o *Options) Complete() error {
	return o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, nil)
}

func (o *Options) Validate() error {
	if o.RequestHeaderClientCAFile == "" {
		return fmt.Errorf("--requestheader-client-ca-file is required")
	}
	return utilerrors(o.SecureServing.Validate())
}

func utilerrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func (o *Options) authenticator(pool *cryptox509.CertPool) authenticator.Request {
	verify := func() (cryptox509.VerifyOptions, bool) {
		return cryptox509.VerifyOptions{
			Roots:     pool,
			KeyUsages: []cryptox509.ExtKeyUsage{cryptox509.ExtKeyUsageClientAuth},
		}, true
	}

	ra := headerrequest.NewDynamicVerifyOptionsSecure(
		x509request.VerifyOptionFunc(verify),
		headerrequest.StaticStringSlice(o.RequestHeaderAllowedNames),
		headerrequest.StaticStringSlice(o.RequestHeaderUsernameHeaders),
		headerrequest.StaticStringSlice(nil), // uid headers
		headerrequest.StaticStringSlice(o.RequestHeaderGroupHeaders),
		headerrequest.StaticStringSlice(o.RequestHeaderExtraHeaderPrefixes),
	)
	// RequestHeader identities are authenticated; add system:authenticated.
	return group.NewAuthenticatedGroupAdder(ra)
}

// New builds a generic apiserver serving the given v1alpha1 storages, keyed by resource name (e.g. "accessibleworkspaces").
func New(name string, o *Options, storages map[string]rest.Storage) (*genericapiserver.GenericAPIServer, error) {
	serverConfig := genericapiserver.NewConfig(Codecs)
	serverConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()

	namer := openapinamer.NewDefinitionNamer(Scheme)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, namer)
	serverConfig.OpenAPIConfig.Info.Title = "kcp-marketplace"
	serverConfig.OpenAPIConfig.Info.Version = "v0.1alpha1"
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, namer)
	serverConfig.OpenAPIV3Config.Info.Title = "kcp-marketplace"
	serverConfig.OpenAPIV3Config.Info.Version = "v0.1alpha1"

	if err := o.SecureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}

	pool, err := cert.NewPool(o.RequestHeaderClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load requestheader client CA %q: %w", o.RequestHeaderClientCAFile, err)
	}

	// Make the TLS server request the front-proxy client cert so the
	// RequestHeader authenticator can verify it before trusting X-Remote-* headers.
	clientCA, err := dynamiccertificates.NewDynamicCAContentFromFile("requestheader-client-ca", o.RequestHeaderClientCAFile)
	if err != nil {
		return nil, err
	}

	serverConfig.SecureServing.ClientCA = clientCA
	serverConfig.Authentication.Authenticator = o.authenticator(pool)

	// Content is filtered per-user by SAR in the storage; any authenticated user
	// may reach the list endpoint. Unauthenticated requests are rejected by authn.
	serverConfig.Authorization.Authorizer = authorizerfactory.NewAlwaysAllowAuthorizer()

	if prefix := strings.TrimRight(o.PathPrefix, "/"); prefix != "" {
		serverConfig.BuildHandlerChainFunc = func(apiHandler http.Handler, c *genericapiserver.Config) http.Handler {
			chain := genericapiserver.DefaultBuildHandlerChain(apiHandler, c)
			stripped := http.StripPrefix(prefix, chain)
			// Only strip for front-proxy-forwarded requests; internal loopback and
			// health probes hit unprefixed paths and must pass through untouched.
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, prefix+"/") || r.URL.Path == prefix {
					stripped.ServeHTTP(w, r)
					return
				}
				chain.ServeHTTP(w, r)
			})
		}
	}

	completed := serverConfig.Complete(nil)
	server, err := completed.New(name, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(marketplacev1alpha1.GroupName, Scheme, ParameterCodec, Codecs)
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = storages
	if err := server.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
