// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsprometheusremotewriteexporter

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

const defaultAMPSigV4Service = "aps"

// signingRoundTripper is a Custom RoundTripper that performs AWS Sig V4.
type signingRoundTripper struct {
	transport   http.RoundTripper
	signer      *v4.Signer
	region      string
	service     string
	runtimeInfo string
}

func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(reqBody)
	reqBody.Close()
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(content)

	// Clone request to ensure thread safety.
	req2 := cloneRequest(req)

	// Add the runtime information to the User-Agent header of the request
	ua := req2.Header.Get("User-Agent")
	if len(ua) > 0 {
		ua = ua + " " + si.runtimeInfo
	} else {
		ua = si.runtimeInfo
	}
	req2.Header.Set("User-Agent", ua)

	// Sign the request
	_, err = si.signer.Sign(req2, body, si.service, si.region, time.Now())
	if err != nil {
		return nil, err
	}

	// Send the request to Prometheus Remote Write Backend.
	resp, err := si.transport.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func newSigningRoundTripper(cfg *Config, next http.RoundTripper, runtimeInfo string) (http.RoundTripper, error) {
	auth := cfg.AuthConfig
	if auth.Region == "" {
		region, err := parseEndpointRegion(cfg.Config.HTTPClientSettings.Endpoint)
		if err != nil {
			return next, err
		}
		auth.Region = region
	}
	if auth.Service == "" {
		auth.Service = defaultAMPSigV4Service
	}

	creds, err := getCredsFromConfig(auth)
	if err != nil {
		return next, err
	}
	return newSigningRoundTripperWithCredentials(auth, creds, next, runtimeInfo)
}

func getCredsFromConfig(auth AuthConfig) (*credentials.Credentials, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{Region: aws.String(auth.Region)},
	})
	if err != nil {
		return nil, err
	}
	if auth.RoleArn != "" {
		// Get credentials from an assumeRole API call.
		return stscreds.NewCredentials(sess, auth.RoleArn, func(p *stscreds.AssumeRoleProvider) {
			p.RoleSessionName = "aws-otel-collector-" + strconv.FormatInt(time.Now().Unix(), 10)
		}), nil
	}
	// Get Credentials, either from ./aws or from environmental variables.
	return sess.Config.Credentials, nil
}

func parseEndpointRegion(endpoint string) (region string, err error) {
	// Example: https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write
	const nDomains = 3
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	p := strings.SplitN(u.Host, ".", nDomains)
	if len(p) < nDomains {
		return "", fmt.Errorf("invalid endpoint: %q", endpoint)
	}
	return p[1], nil
}

func newSigningRoundTripperWithCredentials(auth AuthConfig, creds *credentials.Credentials, next http.RoundTripper, runtimeInfo string) (http.RoundTripper, error) {
	if creds == nil {
		return nil, errors.New("no AWS credentials exist")
	}
	signer := v4.NewSigner(creds)
	rt := signingRoundTripper{
		transport:   next,
		signer:      signer,
		region:      auth.Region,
		service:     auth.Service,
		runtimeInfo: runtimeInfo,
	}
	return &rt, nil
}

func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
