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

// Package awsprometheusremotewriteexporter provides a Prometheus Remote Write Exporter with AWS Sigv4 authentication
package awsprometheusremotewriteexporter

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// signingRoundTripper is a Custom RoundTripper that performs AWS Sig V4
type signingRoundTripper struct {
	transport http.RoundTripper
	signer    *v4.Signer
	region    string
	service   string
}

// RoundTrip signs each outgoing request
func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	// Get the body
	content, err := ioutil.ReadAll(reqBody)
	if err != nil {
		return nil, err
	}

	body := bytes.NewReader(content)

	// Clone request to ensure thread safety
	req2 := cloneRequest(req)

	// Sign the request
	_, err = si.signer.Sign(req2, body, si.service, si.region, time.Now())
	if err != nil {
		return nil, err
	}

	// Send the request to Prometheus Remote Write Backend
	resp, err := si.transport.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func newSigningRoundTripper(auth AuthConfig, next http.RoundTripper) (http.RoundTripper, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(auth.Region)},
	)
	if err != nil {
		return nil, err
	}

	if _, err = sess.Config.Credentials.Get(); err != nil {
		return nil, err
	}

	// Get Credentials, either from ./aws or from environmental variables
	creds := sess.Config.Credentials

	return createSigningRoundTripperWithCredentials(auth, creds, next)
}

func createSigningRoundTripperWithCredentials(auth AuthConfig, creds *credentials.Credentials, next http.RoundTripper) (http.RoundTripper, error) {
	if !isValidAuth(auth) {
		return next, nil
	}

	if creds == nil {
		return nil, errors.New("no AWS credentials exist")
	}

	signer := v4.NewSigner(creds)

	rt := signingRoundTripper{
		transport: next,
		signer:    signer,
		region:    auth.Region,
		service:   auth.Service,
	}

	// return a RoundTripper
	return &rt, nil
}

func isValidAuth(params AuthConfig) bool {
	return params.Region != "" && params.Service != ""
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
