// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	k8sruntime.ReallyCrash = false
	k8sruntime.PanicHandlers = []func(context.Context, any){}
}

// AuthType describes the type of authentication to use for the K8s API
type AuthType string

// TODO: Add option for TLS once
// https://go.opentelemetry.io/collector/issues/933
// is addressed.
const (
	// AuthTypeNone means no auth is required
	AuthTypeNone AuthType = "none"
	// AuthTypeServiceAccount means to use the built-in service account that
	// K8s automatically provisions for each pod.
	AuthTypeServiceAccount AuthType = "serviceAccount"
	// AuthTypeKubeConfig uses local credentials like those used by kubectl.
	AuthTypeKubeConfig AuthType = "kubeConfig"
	// AuthTypeTLS indicates that client TLS auth is desired
	AuthTypeTLS AuthType = "tls"
)

var authTypes = map[AuthType]bool{
	AuthTypeNone:           true,
	AuthTypeServiceAccount: true,
	AuthTypeKubeConfig:     true,
	AuthTypeTLS:            true,
}

// APIConfig contains options relevant to connecting to the K8s API
type APIConfig struct {
	// How to authenticate to the K8s API server.  This can be one of `none`
	// (for no auth), `serviceAccount` (to use the standard service account
	// token provided to the agent pod), or `kubeConfig` to use credentials
	// from `~/.kube/config`.
	AuthType AuthType `mapstructure:"auth_type"`

	// When using auth_type `kubeConfig`, override the current context.
	Context string `mapstructure:"context"`
}

// Validate validates the K8s API config
func (c APIConfig) Validate() error {
	if !authTypes[c.AuthType] {
		return fmt.Errorf("invalid authType for kubernetes: %v", c.AuthType)
	}

	return nil
}

// CreateRestConfig creates an Kubernetes API config from user configuration.
func CreateRestConfig(apiConf APIConfig) (*rest.Config, error) {
	var authConf *rest.Config
	var err error

	authType := apiConf.AuthType

	var k8sHost string
	if authType != AuthTypeKubeConfig {
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 || len(port) == 0 {
			return nil, fmt.Errorf("unable to load k8s config, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
		}
		k8sHost = "https://" + net.JoinHostPort(host, port)
	}

	switch authType {
	case AuthTypeKubeConfig:
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		if apiConf.Context != "" {
			configOverrides.CurrentContext = apiConf.Context
		}
		authConf, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, configOverrides).ClientConfig()

		if err != nil {
			return nil, fmt.Errorf("error connecting to k8s with auth_type=%s: %w", AuthTypeKubeConfig, err)
		}
	case AuthTypeNone:
		authConf = &rest.Config{
			Host: k8sHost,
		}
		authConf.Insecure = true
	case AuthTypeServiceAccount:
		// This should work for most clusters but other auth types can be added
		authConf, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	authConf.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		// Don't use system proxy settings since the API is local to the
		// cluster
		if t, ok := rt.(*http.Transport); ok {
			t.Proxy = nil
		}
		return rt
	}

	return authConf, nil
}

// MakeClient can take configuration if needed for other types of auth
func MakeClient(apiConf APIConfig) (k8s.Interface, error) {
	if err := apiConf.Validate(); err != nil {
		return nil, err
	}

	authConf, err := CreateRestConfig(apiConf)
	if err != nil {
		return nil, err
	}

	client, err := k8s.NewForConfig(authConf)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// MakeDynamicClient can take configuration if needed for other types of auth
func MakeDynamicClient(apiConf APIConfig) (dynamic.Interface, error) {
	if err := apiConf.Validate(); err != nil {
		return nil, err
	}

	authConf, err := CreateRestConfig(apiConf)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(authConf)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// MakeOpenShiftQuotaClient can take configuration if needed for other types of auth
// and return an OpenShift quota API client
func MakeOpenShiftQuotaClient(apiConf APIConfig) (quotaclientset.Interface, error) {
	if err := apiConf.Validate(); err != nil {
		return nil, err
	}

	authConf, err := CreateRestConfig(apiConf)
	if err != nil {
		return nil, err
	}

	client, err := quotaclientset.NewForConfig(authConf)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func NewNodeSharedInformer(client k8s.Interface, nodeName string, watchSyncPeriod time.Duration) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.CoreV1().Nodes().List(context.Background(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.CoreV1().Nodes().Watch(context.Background(), opts)
			},
		},
		&api_v1.Node{},
		watchSyncPeriod,
	)
	return informer
}
