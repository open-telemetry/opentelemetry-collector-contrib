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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const throttleDuration = time.Minute

type client struct {
	config         *Config
	client         *http.Client
	logger         *zap.Logger
	wg             sync.WaitGroup
	throttled      bool
	throttleTimer  *time.Timer
	throttleLock   sync.RWMutex
	timesThrottled int
	buildVersion   string
}

func (c *client) sendLogs(
	ctx context.Context,
	ld pdata.Logs,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	if c.isThrottled() {
		return consumererror.NewPermanent(errors.New("observIQ still throttled due to a previous request"))
	}

	// Conversion errors should be returned after sending what could be converted.
	data, conversionErrs := logdataToObservIQFormat(ld, c.config.AgentID, c.config.AgentName, c.buildVersion)

	jsonData, err := json.Marshal(data)

	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to marshal log data to json, logs: %v", err))
	}

	var gzipped bytes.Buffer
	gzipper := gzip.NewWriter(&gzipped)
	if _, err = gzipper.Write(jsonData); err != nil {
		return consumererror.NewPermanent(err)
	}

	if err = gzipper.Close(); err != nil {
		return consumererror.NewPermanent(err)
	}

	request, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, &gzipped)
	if err != nil {
		return err
	}

	if c.config.APIKey != "" {
		request.Header.Set("x-cabin-api-key", c.config.APIKey)
	} else {
		// Secret key is configured instead of api key
		request.Header.Set("x-cabin-secret-key", c.config.SecretKey)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("x-cabin-agent-id", c.config.AgentID)
	request.Header.Set("x-cabin-agent-name", c.config.AgentName)
	request.Header.Set("x-cabin-agent-version", c.buildVersion)

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
		return consumererror.NewPermanent(fmt.Errorf("request to observIQ returned a 400, skipping this chunk. body: %s", body))

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
		return consumererror.NewPermanent(
			fmt.Errorf("unable to send logs to observIQ (status code: %d), please check your account. Body: %s", res.StatusCode, body))

	case res.StatusCode > 405:
		return fmt.Errorf("request to observIQ returned a failure code. status_code %d, status %s", res.StatusCode, res.Status)
	}

	if len(conversionErrs) > 0 {
		return multierr.Combine(conversionErrs...)
	}

	return nil
}

func (c *client) isThrottled() bool {
	c.throttleLock.RLock()
	throttled := c.throttled
	c.throttleLock.RUnlock()

	return throttled
}

/*
	Throttling causes logs to be dropped for a period of time (see throttleDuration).
	Throttling occurs in response to various issues, such as authentication errors or other account issues
*/
func (c *client) throttle() {
	c.throttleLock.Lock()
	defer c.throttleLock.Unlock()

	if c.throttleTimer != nil {
		c.throttleTimer.Stop()
	}

	c.timesThrottled++
	c.throttled = true

	// Specify to only clear the throttle if we don't throttle again in the meantime
	curTimesThrottled := c.timesThrottled
	clearThrottleFunc := func() {
		c.clearThrottle(curTimesThrottled)
	}

	c.throttleTimer = timeAfterFunc(throttleDuration, clearThrottleFunc)
}

/*
	Clears current throttle.
	throttleNum indicates which instance of the throttle to clear;
	this should be equal to c.throttleTimes.
*/
func (c *client) clearThrottle(throttleNum int) {
	c.throttleLock.Lock()
	defer c.throttleLock.Unlock()
	// Prevent case where the timer has fired, but another throttle occurred between the timer firing and the lock being acquired
	if throttleNum == c.timesThrottled {
		c.throttled = false
	}
}

func (c *client) stop(context.Context) error {
	c.wg.Wait() // Wait for all sends to finish
	return nil
}

func buildClient(config *Config, logger *zap.Logger, buildInfo component.BuildInfo) (*client, error) {
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
		logger:       logger,
		config:       config,
		buildVersion: buildInfo.Version,
	}, nil
}
