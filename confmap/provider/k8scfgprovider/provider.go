// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package k8scfgprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/k8scfgprovider"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

const schemeName = "k8scfg"

type provider struct {
	// Add k8s credentials
	logger *zap.Logger
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(settings confmap.ProviderSettings) confmap.Provider {
	l := settings.Logger

	if l == nil {
		l = zap.NewNop()
	}
	return &provider{
		logger: l.With(zap.String("provider", schemeName)),
	}
}

func getFromConfigMap(ctx context.Context, spec *valueSpec) ([]byte, error) {
	cm, err := getConfigMap(ctx, spec.namespace, spec.name)
	if err != nil {
		return nil, err
	}

	var res []byte
	switch spec.dataType {
	case "data":
		res = []byte(cm.Data[spec.key])
	case "binaryData":
		res = cm.BinaryData[spec.key]
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("%s[%s] is empty", spec.dataType, spec.key)
	}
	return res, nil
}

func getFromSecret(ctx context.Context, spec *valueSpec) ([]byte, error) {
	s, err := getSecret(ctx, spec.namespace, spec.name)
	if err != nil {
		return nil, err
	}

	var res []byte
	switch spec.dataType {
	case "data":
		res = s.Data[spec.key]
	case "stringData":
		res = []byte(s.StringData[spec.key])
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("%s[%s] is empty", spec.dataType, spec.key)
	}
	return res, nil
}

// Retrieve gets the data from either a secret or a config map using the following schema
//
//	k8sconfig:configMap:namespace:<configmap name>:<data or binaryData>:<item>  for a configmap
//	k8sconfig:secret:namespace:<secret name>:<data or stringData>:<item>  for a secret
func (p *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	// use uri to parse
	spec, err := parseURI(uri)
	if err != nil {
		err = fmt.Errorf("error parsing uri: %w", err)
		p.logger.Error("parsing error", zap.Error(err))
		return nil, err
	}

	var res []byte
	switch strings.ToLower(spec.kind) {
	case "configmap":
		res, err = getFromConfigMap(ctx, spec)
	case "secret":
		res, err = getFromSecret(ctx, spec)
	}
	if err != nil {
		p.logger.Error("error retrieving the value", zap.Error(err))
		return nil, err
	}

	return confmap.NewRetrievedFromYAML(res)
}

func (p *provider) Scheme() string {
	p.logger.Info("returning scheme", zap.String("scheme", schemeName))
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
