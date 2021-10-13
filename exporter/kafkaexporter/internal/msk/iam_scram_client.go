// Copyright  The OpenTelemetry Authors
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
package msk

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"
	"go.uber.org/multierr"

	"github.com/aws/aws-sdk-go/aws/credentials"
	sign "github.com/aws/aws-sdk-go/aws/signer/v4"
)

const (
	Mechanism = "AWS_MSK_IAM"

	service          = "kafka-cluster"
	supportedVersion = "2020_10_22"
)

const (
	_ int32 = iota // Ignoring the zero value to ensure we start up correctly
	initMessage
	serverResponse
	complete
	failed
)

var (
	ErrFailedServerChallenge = errors.New("failed server challenge")
	ErrBadChallenge          = errors.New("invalid challenge data provided")
	ErrInvalidStateReached   = errors.New("invalid state reached")
)

type IAMSASLClient struct {
	MSKHostname string
	Region      string
	UserAgent   string

	signer *sign.StreamSigner

	state     int32
	accessKey string
	secretKey string
}

type mskResponse struct {
	Version   string `json:"version"`
	RequestID string `json:"request-id"`
}

var _ sarama.SCRAMClient = (*IAMSASLClient)(nil)

func NewIAMSASLClient(MSKHostname, region, useragent string) sarama.SCRAMClient {
	return &IAMSASLClient{
		MSKHostname: MSKHostname,
		Region:      region,
		UserAgent:   useragent,
	}
}

func (sc *IAMSASLClient) Begin(username, password, _ string) error {
	if sc.MSKHostname == "" {
		return errors.New("missing required MSK Broker hostname")
	}

	if sc.Region == "" {
		return errors.New("missing MSK cluster region")
	}

	if sc.UserAgent == "" {
		return errors.New("missing value for MSK user agent")
	}

	sc.signer = sign.NewStreamSigner(
		sc.Region,
		service,
		nil,
		credentials.NewStaticCredentials(
			username,
			password,
			"",
		),
	)
	sc.accessKey = username
	sc.secretKey = password
	sc.state = initMessage
	return nil
}

func (sc *IAMSASLClient) Step(challenge string) (string, error) {
	var resp string

	switch atomic.LoadInt32(&sc.state) {
	case initMessage:
		if challenge != "" {
			atomic.StoreInt32(&sc.state, failed)
			return "", fmt.Errorf("challenge must be empty for initial request: %w", ErrBadChallenge)
		}
		payload, err := sc.getAuthPayload()
		if err != nil {
			atomic.StoreInt32(&sc.state, failed)
			return "", err
		}
		resp = string(payload)
		atomic.StoreInt32(&sc.state, serverResponse)
	case serverResponse:
		if challenge == "" {
			atomic.StoreInt32(&sc.state, failed)
			return "", fmt.Errorf("challenge must not be empty for server resposne: %w", ErrBadChallenge)
		}

		decoder := json.NewDecoder(strings.NewReader(challenge))
		decoder.DisallowUnknownFields()

		var resp mskResponse
		if err := decoder.Decode(&resp); err != nil {
			atomic.StoreInt32(&sc.state, failed)
			return "", fmt.Errorf("unable to process msk challenge response: %w", multierr.Combine(err, ErrFailedServerChallenge))
		}
		if resp.Version != supportedVersion {
			atomic.StoreInt32(&sc.state, failed)
			return "", fmt.Errorf("unknown version found in response: %w", ErrFailedServerChallenge)
		}

		atomic.StoreInt32(&sc.state, complete)
	default:
		return "", fmt.Errorf("invalid invocation: %w", ErrInvalidStateReached)
	}

	return resp, nil
}

func (sc *IAMSASLClient) Done() bool { return atomic.LoadInt32(&sc.state) == complete }

func (sc *IAMSASLClient) getAuthPayload() ([]byte, error) {
	timestamp := time.Now().UTC()

	headers := []byte("host:" + sc.MSKHostname)

	sig, err := sc.signer.GetSignature(headers, nil, timestamp)
	if err != nil {
		return nil, err
	}

	auth := map[string]string{
		"version":             supportedVersion,
		"host":                sc.MSKHostname,
		"user-agent":          sc.UserAgent,
		"action":              "kafka-cluster:Connect",
		"x-amz-algorithm":     "AWS4-HMAC-SHA256",
		"x-amz-credential":    path.Join(sc.accessKey, timestamp.Format("20060102T150405Z")[:8], sc.Region, "/kafka-cluster/aws4_request"),
		"x-amz-date":          timestamp.Format("20060102T150405Z"),
		"x-amz-signedheaders": "host",
		"x-amz-expires":       "300",
		"x-amz-signature":     string(sig),
	}

	return json.Marshal(auth)
}
