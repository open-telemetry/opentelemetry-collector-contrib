// Copyright 2020, OpenTelemetry Authors
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

package kubelet

import (
	"crypto/x509"
	"io/ioutil"

	"github.com/pkg/errors"
)

func systemCertPoolPlusPath(certPath string) (*x509.CertPool, error) {
	sysCerts, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.WithMessage(err, "Could not load system x509 cert pool")
	}
	return certPoolPlusPath(sysCerts, certPath)
}

func certPoolPlusPath(certPool *x509.CertPool, certPath string) (*x509.CertPool, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Cert path %s could not be read", certPath)
	}
	ok := certPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, errors.New("AppendCertsFromPEM failed")
	}
	return certPool, nil
}
