// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

type UAATokenProvider struct {
	client         *uaago.Client
	logger         *zap.Logger
	username       string
	password       string
	tlsSkipVerify  bool
	cachedToken    string
	expirationTime *time.Time
	mutex          *sync.Mutex
}

func newUAATokenProvider(logger *zap.Logger, config LimitedHTTPClientSettings, username string, password string) (*UAATokenProvider, error) {
	client, err := uaago.NewClient(config.Endpoint)
	if err != nil {
		return nil, err
	}

	logger.Debug(fmt.Sprintf("creating new cloud foundry UAA token client with url %s username %s", config.Endpoint, username))

	return &UAATokenProvider{
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

func (utp *UAATokenProvider) ProvideToken() (string, error) {
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
