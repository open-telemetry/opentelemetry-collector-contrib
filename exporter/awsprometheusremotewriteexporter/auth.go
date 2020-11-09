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
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// SigningRoundTripper is a Custom RoundTripper that performs AWS Sig V4
type SigningRoundTripper struct {
	transport http.RoundTripper
	signer    *v4.Signer
	cfg       *aws.Config
	service   string
}

// RoundTrip signs each outgoing request
func (si *SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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

	// Sign the request
	_, err = si.signer.Sign(req, body, si.service, *si.cfg.Region, time.Now())
	if err != nil {
		return nil, err
	}

	// Send the request to Prometheus Remote Write Backend
	resp, err := si.transport.RoundTrip(req)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return resp, err
}

func signingRoundTripper(roundTripper http.RoundTripper) (http.RoundTripper, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion)},
		aws.NewConfig().WithLogLevel(aws.LogDebugWithSigning),
	)
	if err != nil {
		return nil, err
	}

	if _, err = sess.Config.Credentials.Get(); err != nil {
		return nil, err
	}

	// Get Credentials, either from ./aws or from environmental variables
	creds := sess.Config.Credentials
	signer := v4.NewSigner(creds)
	rtp := SigningRoundTripper{
		transport: roundTripper,
		signer:    signer,
		cfg:       sess.Config,
		service:   awsService,
	}
	// return a RoundTripper
	return &rtp, nil
}
