// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"

import (
	"context"
	"errors"
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
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
)

func init() {
	k8sruntime.ReallyCrash = false
	k8sruntime.PanicHandlers = []func(context.Context, any){}
}

// AuthType describes the type of authentication to use for the K8s API.
// It is defined in the light k8sconfigtypes package so config schemas can
// reference it without depending on k8s.io/client-go.
type AuthType = k8sconfigtypes.AuthType

const (
	// AuthTypeNone means no auth is required
	AuthTypeNone = k8sconfigtypes.AuthTypeNone
	// AuthTypeServiceAccount means to use the built-in service account that
	// K8s automatically provisions for each pod.
	AuthTypeServiceAccount = k8sconfigtypes.AuthTypeServiceAccount
	// AuthTypeKubeConfig uses local credentials like those used by kubectl.
	AuthTypeKubeConfig = k8sconfigtypes.AuthTypeKubeConfig
	// AuthTypeTLS indicates that client TLS auth is desired
	AuthTypeTLS = k8sconfigtypes.AuthTypeTLS
)

const (
	// DefaultKubeAPIQPS is the default number of queries per second to the Kubernetes API.
	// Matches client-go's built-in default.
	DefaultKubeAPIQPS float32 = 5
	// DefaultKubeAPIBurst is the default burst limit for requests to the Kubernetes API.
	// Matches client-go's built-in default.
	DefaultKubeAPIBurst int = 10
)

// APIConfig contains options relevant to connecting to the K8s API.
// The alias preserves the parent package's existing API and type identity.
type APIConfig = k8sconfigtypes.APIConfig

// CreateRestConfig creates an Kubernetes API config from user configuration.
func CreateRestConfig(apiConf APIConfig) (*rest.Config, error) {
	var authConf *rest.Config
	var err error

	authType := apiConf.AuthType

	var k8sHost string
	if authType != AuthTypeKubeConfig {
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		if host == "" || port == "" {
			return nil, errors.New("unable to load k8s config, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
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
			loadingRules, configOverrides,
		).ClientConfig()
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

	// Don't use system proxy settings since the API is local to the
	// cluster
	authConf.Proxy = http.ProxyURL(nil)

	if apiConf.KubeAPIQPS > 0 {
		authConf.QPS = apiConf.KubeAPIQPS
	}
	if apiConf.KubeAPIBurst > 0 {
		authConf.Burst = apiConf.KubeAPIBurst
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

// ClientBundle groups the two Kubernetes clients:
//
//   - K8s (typed client): kubernetes.Interface for full resource objects
//     (spec/status/metadata). Use when you need complete data or typed informers.
//
//   - Meta (metadata client): metadata.Interface for PartialObjectMetadata
//     (name/namespace/UID/labels/annotations/ownerRefs). Use for lightweight
//     list/watch when only metadata is needed (e.g., high-churn resources).
type ClientBundle struct {
	K8s  k8s.Interface
	Meta metadata.Interface
}

// MakeClientBundle builds both clients from a single RestConfig,
// ensuring shared auth/transport. In unit tests, inject a fake
// metadata client (metadata/fake) to avoid network calls, while
// typed resources can use kubernetes/fake.
func MakeClientBundle(apiConf APIConfig) (ClientBundle, error) {
	if err := apiConf.Validate(); err != nil {
		return ClientBundle{}, err
	}

	rc, err := CreateRestConfig(apiConf)
	if err != nil {
		return ClientBundle{}, err
	}

	kc, err := k8s.NewForConfig(rc)
	if err != nil {
		return ClientBundle{}, err
	}

	mc, err := metadata.NewForConfig(rc)
	if err != nil {
		return ClientBundle{}, err
	}

	return ClientBundle{K8s: kc, Meta: mc}, nil
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
			ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.CoreV1().Nodes().List(ctx, opts)
			},
			WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.CoreV1().Nodes().Watch(ctx, opts)
			},
		},
		&api_v1.Node{},
		watchSyncPeriod,
	)
	return informer
}
