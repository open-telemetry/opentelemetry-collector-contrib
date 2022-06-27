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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type icingaClientConfig struct {
	Host                   string
	Username               string
	Password               string
	DisableSslVerification bool
	Filter                 string
	logger                 *zap.SugaredLogger
}

type icingaClient struct {
	config icingaClientConfig
	cancel context.CancelFunc
}

type icingaEventStreamRequest struct {
	Types  []string `json:"types"`
	Queue  string   `json:"queue"`
	Filter string   `json:"filter"`
}

func newClient(config icingaClientConfig) (icingaClient, error) {
	client := icingaClient{config: config}
	return client, nil
}

func (u *icingaClient) Listen(
	ctx context.Context,
	parser icingaParser,
	nextConsumer consumer.Metrics,
) error {
	if nextConsumer == nil {
		return component.ErrNilNextConsumer
	}

	waitDuration := 5 * time.Second

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, cancel, err := openConnection(u.config)

				if err != nil {
					u.config.logger.Errorf("Failed establishing a connection. Waiting %s until trying again. Error: %s", waitDuration.String(), err)
					time.Sleep(waitDuration)
					continue
				}
				u.config.logger.Info("Successfully established connection. Listening to events.")
				u.cancel = cancel

				scanner := bufio.NewScanner(res.Body)
				for scanner.Scan() {
					line := scanner.Text()
					parser.Aggregate(line)
					err := nextConsumer.ConsumeMetrics(ctx, parser.getMetrics())
					if err != nil {
						fmt.Printf("Error while consuming metrics %s", err)
					}
				}
				u.config.logger.Infof("Stream ended. Waiting %s and then trying again.", waitDuration.String())
				time.Sleep(waitDuration)
			}
		}
	}()

	return nil
}

func openConnection(config icingaClientConfig) (*http.Response, context.CancelFunc, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.DisableSslVerification},
	}
	client := &http.Client{
		Transport: transport,
	}
	body := &icingaEventStreamRequest{
		Types:  []string{"CheckResult"},
		Queue:  "icingareceiver",
		Filter: config.Filter,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}

	url := "https://" + config.Host + "/v1/events"
	config.logger.Infof("Opening connection to %s to listen for events.", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(buf))

	if err != nil {
		return nil, nil, err
	}

	_, cancel := context.WithCancel(context.Background())

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password)))
	res, err := client.Do(req)

	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != http.StatusOK {
		config.logger.Errorf("Status code %s not OK", strconv.Itoa(res.StatusCode))
	}

	return res, cancel, nil
}

func (u *icingaClient) Close() error {
	if u.cancel != nil {
		u.cancel()
	}
	return nil
}
