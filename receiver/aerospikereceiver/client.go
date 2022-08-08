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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"fmt"
	"strings"
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/aerospike/aerospike-client-go/v5/types"
	"go.uber.org/zap"
)

type defaultASClient struct {
	conn   *as.Connection   // open connection to Aerospike
	policy *as.ClientPolicy // Timeout and authentication information
	logger *zap.Logger      // logs malformed metrics in responses
}

// aerospike is the interface that provides information about a given node
type aerospike interface {
	// NamespaceInfo gets information about a specific namespace
	NamespaceInfo(namespace string) (map[string]string, error)
	// Info gets high-level information about the node/system.
	Info() (map[string]string, error)
	// Close closes the connection to the Aerospike node
	Close()
}

// newASClient creates a new defaultASClient connected to the given host and port.
// If username and password aren't blank, they're used to authenticate
func newASClient(host string, port int, username, password string, timeout time.Duration, logger *zap.Logger) (*defaultASClient, error) {
	authEnabled := username != "" && password != ""

	policy := as.NewClientPolicy()
	policy.Timeout = timeout
	if authEnabled {
		policy.User = username
		policy.Password = password
	}

	conn, err := as.NewConnection(policy, as.NewHost(host, port))
	if err != nil {
		return nil, err
	}

	if authEnabled {
		if err := conn.Login(policy); err != nil {
			return nil, err
		}
	}

	return &defaultASClient{
		conn:   conn,
		logger: logger,
		policy: policy,
	}, nil
}

func (c *defaultASClient) NamespaceInfo(namespace string) (map[string]string, error) {
	if err := c.conn.SetTimeout(time.Now().Add(c.policy.Timeout), c.policy.Timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}
	namespaceKey := "namespace/" + namespace
	response, err := c.conn.RequestInfo(namespaceKey)

	// Try to login and get a new session
	if err != nil && err.Matches(types.EXPIRED_SESSION) {
		if loginErr := c.conn.Login(c.policy); loginErr != nil {
			return nil, loginErr
		}
	}

	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	for k, v := range response {
		if k == namespaceKey {
			for _, pair := range splitFields(v) {
				parts := splitPair(pair)
				if len(parts) != 2 {
					c.logger.Warn(fmt.Sprintf("metric pair '%s' not in key=value format", pair))
					continue
				}
				info[parts[0]] = parts[1]

			}

		}
	}
	info["name"] = namespace
	return info, nil
}

func (c *defaultASClient) Info() (map[string]string, error) {
	if err := c.conn.SetTimeout(time.Now().Add(c.policy.Timeout), c.policy.Timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}

	response, err := c.conn.RequestInfo("namespaces", "node", "statistics", "services")

	// Try to login and get a new session
	if err != nil && err.Matches(types.EXPIRED_SESSION) {
		if loginErr := c.conn.Login(c.policy); loginErr != nil {
			return nil, loginErr
		}
	}

	if err != nil {
		return nil, err
	}
	info := make(map[string]string)
	for k, v := range response {
		switch k {
		case "statistics":
			for _, pair := range splitFields(v) {
				parts := splitPair(pair)
				if len(parts) != 2 {
					c.logger.Warn(fmt.Sprintf("metric pair '%s' not in key=value format", pair))
					continue
				}
				info[parts[0]] = parts[1]

			}
		default:
			info[k] = v
		}
	}
	return info, nil
}

func (c *defaultASClient) Close() {
	c.conn.Close()
}

// splitPair splits a metric pair in the format of 'key=value'
func splitPair(pair string) []string {
	return strings.Split(pair, "=")
}

// splitFields splits a string of metric pairs delimited with the ';' character
func splitFields(info string) []string {
	return strings.Split(info, ";")
}
