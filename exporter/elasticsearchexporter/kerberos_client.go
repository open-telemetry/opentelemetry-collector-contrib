// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"fmt"

	krbclient "github.com/jcmturner/gokrb5/v8/client"
	krbconfig "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
)

func NewKerberosClient(config KerberosSettings) (*krbclient.Client, error) {
	var krbClient *krbclient.Client
	krbConf, err := krbconfig.Load(config.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error creating Kerberos client: %w", err)
	}

	switch config.AuthType {
	case authKeytab:
		kTab, err := keytab.Load(config.KeyTabPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load keytab file %s: %w", config.KeyTabPath, err)
		}
		krbClient = krbclient.NewWithKeytab(config.Username, config.Realm, kTab, krbConf)
	case authPassword:
		krbClient = krbclient.NewWithPassword(config.Username, config.Realm, string(config.Password), krbConf)
	default:
		return nil, ErrInvalidAuthType
	}

	return krbClient, err
}
