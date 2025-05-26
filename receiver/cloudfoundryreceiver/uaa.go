// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/uaago"
	"go.uber.org/zap"
)

const (
	// time added to expiration time to reduce chance of using expired token due to network latency
	expirationTimeBuffer = -5 * time.Second
)

type uaaTokenProvider struct {
	client         *uaago.Client
	logger         *zap.Logger
	username       string
	password       string
	tlsSkipVerify  bool
	cachedToken    string
	expirationTime *time.Time
	mutex          *sync.Mutex
}

func newUAATokenProvider(logger *zap.Logger, config LimitedClientConfig, username string, password string) (*uaaTokenProvider, error) {
	client, err := uaago.NewClient(config.Endpoint)
	if err != nil {
		return nil, err
	}

	logger.Debug(fmt.Sprintf("creating new cloud foundry UAA token client with url %s username %s", config.Endpoint, username))

	return &uaaTokenProvider{
		logger:         logger,
		client:         client,
		username:       username,
		password:       password,
		tlsSkipVerify:  config.TLSSetting.InsecureSkipVerify,
		cachedToken:    "",
		expirationTime: nil,
		mutex:          &sync.Mutex{},
	}, nil
}

func (utp *uaaTokenProvider) ProvideToken() (string, error) {
	utp.mutex.Lock()
	defer utp.mutex.Unlock()

	now := time.Now()

	if utp.expirationTime != nil {
		if utp.expirationTime.Before(now) {
			utp.logger.Debug("cloud foundry UAA token has expired")
			utp.cachedToken = ""
		}
	}

	if utp.cachedToken != "" {
		return utp.cachedToken, nil
	}

	token, expiresInSeconds, err := utp.client.GetAuthTokenWithExpiresIn(utp.username, utp.password, utp.tlsSkipVerify)
	if err != nil {
		return "", fmt.Errorf("get auth token from UAA: %w", err)
	}

	if expiresInSeconds > 0 {
		expirationTime := now.Add(time.Duration(int64(expiresInSeconds) * time.Second.Nanoseconds())).Add(expirationTimeBuffer)
		utp.expirationTime = &expirationTime
		utp.logger.Debug(fmt.Sprintf("received new cloud foundry UAA token which expires in %d seconds", expiresInSeconds))
	} else {
		utp.expirationTime = nil
		utp.logger.Debug("received new cloud foundry UAA token with no expiration time")
	}

	utp.cachedToken = token
	return token, nil
}
