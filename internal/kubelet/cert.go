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
	"errors"
	"fmt"
	"io/ioutil"
)

func systemCertPoolPlusPath(certPath string) (*x509.CertPool, error) {
	sysCerts, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("could not load system x509 cert pool: %w", err)
	}
	return certPoolPlusPath(sysCerts, certPath)
}

func certPoolPlusPath(certPool *x509.CertPool, certPath string) (*x509.CertPool, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("cert path %s could not be read: %w", certPath, err)
	}
	ok := certPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, errors.New("AppendCertsFromPEM failed")
	}
	return certPool, nil
}
