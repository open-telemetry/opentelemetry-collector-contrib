// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"errors"
	"net"
	"os"
	"strings"

	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

const (
	defaultServiceTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"  //#nosec
	defaultCAPath           = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" //#nosec
)

func readK8STokenFromFile() (string, error) {
	token, err := os.ReadFile(defaultServiceTokenPath)
	if err != nil {
		return "", err
	}
	// Trailing whitespace would make the Authorization header value invalid.
	t := strings.TrimSpace(string(token))
	if t == "" {
		return "", errors.New("empty service account token")
	}
	return t, nil
}

func readSVCAddressFromENV() (string, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	if host == "" {
		return "", errors.New("could not extract openshift api host")
	}
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		return "", errors.New("could not extract openshift api port")
	}
	// JoinHostPort brackets the bare IPv6 host Kubernetes sets on IPv6 clusters.
	return "https://" + net.JoinHostPort(host, port), nil
}

// Config can contain user-specified inputs to overwrite default values.
// Unset fields are resolved by the SDK detector from the pod environment.
type Config struct {
	// Address is the address of the openshift api server
	Address string `mapstructure:"address"`

	// Token is used to identify against the openshift api server
	Token string `mapstructure:"token"`

	// TLSs contains TLS configurations that are specific to client
	// connection used to communicate with the OpenShift API.
	TLSs configtls.ClientConfig `mapstructure:"tls"`

	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
