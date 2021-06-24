// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observiqexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const throttleDuration = 1 * time.Minute

type client struct {
	config         *Config
	client         *http.Client
	logger         *zap.Logger
	wg             sync.WaitGroup
	throttled      bool
	throttledUntil time.Time
	throttleLock   sync.RWMutex
	clock          clock
}

func (c *client) sendLogs(
	ctx context.Context,
	ld pdata.Logs,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	if c.isThrottled() {
		return exporterhelper.NewThrottleRetry(errors.New("observIQ still throttled, attempting to delay logs"), c.throttledUntil.Sub(c.clock.Now()))
	}

	// Conversion errors should be returned after sending what could be converted.
	data, conversionErrs := logdataToObservIQFormat(ld, c.config.AgentID, c.config.AgentName, c.clock)

	jsonData, err := json.Marshal(data)

	if err != nil {
		return consumererror.Permanent(fmt.Errorf("failed to marshal log data to json, logs: %v", err))
	}

	var gzipped bytes.Buffer
	gzipper := gzip.NewWriter(&gzipped)
	if _, err = gzipper.Write(jsonData); err != nil {
		return consumererror.Permanent(err)
	}

	if err = gzipper.Close(); err != nil {
		return consumererror.Permanent(err)
	}

	request, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, &gzipped)
	if err != nil {
		return err
	}

	request.Header.Set("x-cabin-agent-name", c.config.AgentName)
	request.Header.Set("x-cabin-api-key", c.config.APIKey)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")

	res, err := c.client.Do(request)

	if err != nil {
		return fmt.Errorf(
			"client failed to submit request to observIQ. "+
				"Review the underlying error message to troubleshoot the issue "+
				"underlying_error: %v", err.Error())
	}

	defer res.Body.Close()

	switch {
	case res.StatusCode == 400:
		// 400: bad request
		// ignore this chunk
		var body string
		bytes, err := io.ReadAll(res.Body)
		if err == nil {
			body = string(bytes)
		}
		return consumererror.Permanent(fmt.Errorf("request to observIQ returned a 400, skipping this chunk. body: %s", body))

	case res.StatusCode >= 401 && res.StatusCode <= 405:
		// 401: unauthorized
		// 402: payment required
		// 403: forbidden
		// 404: not found
		var body string
		bytes, err := io.ReadAll(res.Body)
		if err == nil {
			body = string(bytes)
		}
		c.throttle()
		return exporterhelper.NewThrottleRetry(
			fmt.Errorf("observiq server returned throttling code (%d), please check your account. Body: %s", res.StatusCode, body),
			throttleDuration)

	case res.StatusCode > 405:
		return fmt.Errorf(
			"request to observIQ returned a failure code. "+
				"Review status and status code for further details. "+
				"status_code %d, status %s", res.StatusCode, res.Status)
	}

	if len(conversionErrs) > 0 {
		return consumererror.Combine(conversionErrs)
	}

	return nil
}

func (c *client) isThrottled() bool {
	c.throttleLock.RLock()
	throttled := c.throttled
	c.throttleLock.RUnlock()

	if !throttled {
		return false
	}

	now := c.clock.Now()
	if now.After(c.throttledUntil) {
		c.throttleLock.Lock()
		c.throttled = false
		c.throttleLock.Unlock()

		return false
	}

	return true
}

/*
	Throttling causes logs to be dropped for a period of time (see throttleDuration).
	Throttling occurs in response to various issues, such as authentication errors or other account issues
*/
func (c *client) throttle() {
	c.throttleLock.Lock()
	defer c.throttleLock.Unlock()

	c.throttled = true
	c.throttledUntil = c.clock.Now().Add(throttleDuration)
}

func (c *client) start(context.Context, component.Host) error {
	return nil
}

func (c *client) stop(context.Context) error {
	c.wg.Wait() // Wait for all sends to finish
	return nil
}

func buildClient(config *Config, logger *zap.Logger, clock clock) (*client, error) {
	tlsCfg, err := config.TLSSetting.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	return &client{
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: tlsCfg,
			},
		},
		logger: logger,
		config: config,
		clock:  clock,
	}, nil
}
