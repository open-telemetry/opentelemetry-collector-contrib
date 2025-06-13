// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"errors"
	"fmt"
	"os"

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
	return string(token), nil
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
	return fmt.Sprintf("https://%s:%s", host, port), nil
}

// Config can contain user-specified inputs to overwrite default values.
// See `openshift.go#NewDetector` for more information.
type Config struct {
	// Address is the address of the openshift api server
	Address string `mapstructure:"address"`

	// Token is used to identify against the openshift api server
	Token string `mapstructure:"token"`

	// TLSSettings contains TLS configurations that are specific to client
	// connection used to communicate with the OpenShift API.
	TLSSettings configtls.ClientConfig `mapstructure:"tls"`

	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// MergeWithDefaults fills unset fields with default values.
func (c *Config) MergeWithDefaults() error {
	if c.Token == "" {
		token, err := readK8STokenFromFile()
		if err != nil {
			return err
		}
		c.Token = token
	}

	if c.Address == "" {
		addr, err := readSVCAddressFromENV()
		if err != nil {
			return err
		}
		c.Address = addr
	}

	if !c.TLSSettings.Insecure && c.TLSSettings.CAFile == "" {
		c.TLSSettings.CAFile = defaultCAPath
	}
	return nil
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
