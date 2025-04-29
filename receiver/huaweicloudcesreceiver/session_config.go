// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"net/url"
	"strconv"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
)

func createHTTPConfig(cfg huaweiSessionConfig) (*config.HttpConfig, error) {
	if cfg.ProxyAddress == "" {
		return config.DefaultHttpConfig().WithIgnoreSSLVerification(cfg.NoVerifySSL), nil
	}
	proxy, err := configureHTTPProxy(cfg)
	if err != nil {
		return nil, err
	}
	return config.DefaultHttpConfig().WithProxy(proxy), nil
}

func configureHTTPProxy(cfg huaweiSessionConfig) (*config.Proxy, error) {
	proxyURL, err := url.Parse(cfg.ProxyAddress)
	if err != nil {
		return nil, err
	}

	proxy := config.NewProxy().
		WithSchema(proxyURL.Scheme).
		WithHost(proxyURL.Hostname())
	if len(proxyURL.Port()) > 0 {
		if i, err := strconv.Atoi(proxyURL.Port()); err == nil {
			proxy = proxy.WithPort(i)
		}
	}

	// Configure the username and password if the proxy requires authentication
	if len(cfg.ProxyUser) > 0 {
		proxy = proxy.WithUsername(cfg.ProxyUser).WithPassword(cfg.ProxyPassword)
	}
	return proxy, nil
}
