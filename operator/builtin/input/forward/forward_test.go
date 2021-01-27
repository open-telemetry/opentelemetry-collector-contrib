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

package forward

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func TestForwardInput(t *testing.T) {
	cfg := NewForwardInputConfig("test")
	cfg.ListenAddress = "0.0.0.0:0"
	cfg.OutputIDs = []string{"fake"}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	forwardInput := ops[0].(*ForwardInput)

	fake := testutil.NewFakeOutput(t)
	err = forwardInput.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	require.NoError(t, forwardInput.Start())
	defer forwardInput.Stop()

	newEntry := entry.New()
	newEntry.Record = "test"
	newEntry.Timestamp = newEntry.Timestamp.Round(time.Second)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode([]*entry.Entry{newEntry}))

	_, port, err := net.SplitHostPort(forwardInput.ln.Addr().String())
	require.NoError(t, err)

	_, err = http.Post(fmt.Sprintf("http://127.0.0.1:%s", port), "application/json", &buf)
	require.NoError(t, err)

	select {
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be received")
	case e := <-fake.Received:
		require.True(t, newEntry.Timestamp.Equal(e.Timestamp))
		require.Equal(t, newEntry.Record, e.Record)
		require.Equal(t, newEntry.Severity, e.Severity)
		require.Equal(t, newEntry.SeverityText, e.SeverityText)
		require.Equal(t, newEntry.Labels, e.Labels)
		require.Equal(t, newEntry.Resource, e.Resource)
	}
}

func TestForwardInputTLS(t *testing.T) {
	certFile, keyFile := createCertFiles(t)

	cfg := NewForwardInputConfig("test")
	cfg.ListenAddress = "0.0.0.0:0"
	cfg.TLS = &TLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	cfg.OutputIDs = []string{"fake"}

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	forwardInput := ops[0].(*ForwardInput)

	fake := testutil.NewFakeOutput(t)
	err = forwardInput.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	require.NoError(t, forwardInput.Start())
	defer forwardInput.Stop()

	newEntry := entry.New()
	newEntry.Record = "test"
	newEntry.Timestamp = newEntry.Timestamp.Round(time.Second)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode([]*entry.Entry{newEntry}))

	_, port, err := net.SplitHostPort(forwardInput.ln.Addr().String())
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(publicCrt)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	_, err = client.Post(fmt.Sprintf("https://127.0.0.1:%s", port), "application/json", &buf)
	require.NoError(t, err)

	select {
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be received")
	case e := <-fake.Received:
		require.True(t, newEntry.Timestamp.Equal(e.Timestamp))
		require.Equal(t, newEntry.Record, e.Record)
		require.Equal(t, newEntry.Severity, e.Severity)
		require.Equal(t, newEntry.SeverityText, e.SeverityText)
		require.Equal(t, newEntry.Labels, e.Labels)
		require.Equal(t, newEntry.Resource, e.Resource)
	}
}

func createCertFiles(t *testing.T) (cert, key string) {
	tempDir := testutil.NewTempDir(t)

	certFile, err := os.Create(filepath.Join(tempDir, "cert"))
	require.NoError(t, err)
	_, err = certFile.Write(publicCrt)
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.Create(filepath.Join(tempDir, "key"))
	require.NoError(t, err)
	_, err = keyFile.Write(privateKey)
	require.NoError(t, err)
	keyFile.Close()

	return certFile.Name(), keyFile.Name()
}

/*
 openssl req -x509 -nodes -newkey rsa:2048 -keyout key.pem -out cert.pem -config san.cnf
 Generated with the following san.cnf:

 [req]
 default_bits  = 2048
 distinguished_name = req_distinguished_name
 req_extensions = req_ext
 x509_extensions = v3_req
 prompt = no

 [req_distinguished_name]
 countryName = XX
 stateOrProvinceName = N/A
 localityName = N/A
 organizationName = Self-signed certificate
 commonName = 120.0.0.1: Self-signed certificate

 [req_ext]
 subjectAltName = @alt_names

 [v3_req]
 subjectAltName = @alt_names

 [alt_names]
 IP.1 = 127.0.0.1
*/
var privateKey = []byte(`
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDVYJKyB9jDamy5
gAdRldyqdFfEoQmWqVFVsNcYVqkbLSagcwoi21p8YU4TMkC/vrkN/jHg9lOiwi5z
Uegt/52HG6cpaaAW3AVwA+4/Cwhi4a/dwUGBO80HMvMQ9RwH2ISLa1X0rWPxVT+l
VNQXxfrEU8Jwv80cDwpJUcywmGWuiiPAYMAZLnQjp3jAS/rbVBV4kWAOlgYstKg9
XjXpssF+LaRGU9F3WMlPtaXuQpMAy54r5nRYVwYGkXmePqC3BQGqgIBkXiaCgSZ5
VLFM5zKGCJVyNwOytsH9exTo/UbbQnClREaHU6q/lIa7yBiTLvbid3ck89xHnX3a
/K8/ODCXAgMBAAECggEBALSppOsp66VheZcCSLASRBjqktmAQ/8VczErnqMT1PCW
pQra/G0Q7qc7OADW3q26zTKE1DSWO7Al23B2nDA+KmGXz0woC4zvU4dJPLKSI9Kd
JeuLUmwadvkucVEdR1N5RphJFCkrmeBe/pl8nmtWjIEoLgyKyR6FuX7kzHuFPSqu
bdkeA91nJ9o1uO7xhAKRBV2tnHL87TvZpQ3fJkJYg4f6cvgaHR8hggGxWqXcBuqh
XMialoXWlYXMXWBrR8sI/NWVTJjX8GMnuOK+qIRb4eSXrMvicLyE/elbd4eOUx2N
WSdA0OeguYNyqgrnb2nsDdFRAmmXdP0mQNrH2CPyNEECgYEA6pi07tpvl+TyYwkj
BA9pvmYbO4LyNyne0ALYB9bA5H7uhYIZ6QhdDRq/3hADrb+3Zt7HDSljjp9D/jRL
tWysa/1N0DX0VDQMiq5q7qcMrNvq8yCuhwt1/y15fXrPMaKOqiO/aV+NhTISqXF4
BtXSD957MV/MbBuaMHJdlp6NzGECgYEA6NhDhihz8Kmo/MyiSBT3QPJwaYPSpOZb
SWe/5jKUrgigPpCkWHjOp+D1vgRskzkBBsWPyuhNuUrKpXGv93RvrjtSWsjFMPwe
DKRXbpD2MpA/7LMmUUNnVln8X8gsEXwv0OlJXftAJWnfH+XT9GjsiVrKLO2Z+I3g
52YZaQ+QX/cCgYA/ZRDP0vuBAn91v8xUlo2uxAzr4hDuU0RA1ePnCmOJ27s9HNE/
peDvX6ElsxIra7l19RG5PswGiIdpNFyZJErby9GxSENEVeRlvYhsAXxtYeh11wkS
uUgjsvg3rm47LYB7/bkGEqo9qjBc1arnvfRMEYUc7JRjSno6SU19HE+ZQQKBgGKO
h6ZVoR2Q8rJue5I/LZkUBXjkD1k1GBauD5AEgOJZTFqvJqE8IVz1346amMqIKmMP
ZJniUmPHwJbe1DjN1CfPfEBpEu51CNMZDNkECvHEFQq/mcxz1125oRV5yQ1tn1+y
HxfkrXYopgT+ZwThFJ3fDAyQVcfbZgMMOF079URpAoGAMLiz6MKWN2f53kXVL4IP
fnn/6s1SwpyCQVAcYwHez50bMjltPov5r3TGCuazWciOEG46mx38JcTwCktJdZr8
fKYH4NM0PNLDSiOHjvLkujlDJrBs4NwLfABPDbW/2387mqtDYbNO+XfVBF85fuZI
+Xfm7rWrp93+rNxKX2+0A+U=
-----END PRIVATE KEY-----
`)

var publicCrt = []byte(`
-----BEGIN CERTIFICATE-----
MIIDhjCCAm6gAwIBAgIJAJOeoSf2lHXyMA0GCSqGSIb3DQEBCwUAMHgxCzAJBgNV
BAYTAlhYMQwwCgYDVQQIDANOL0ExDDAKBgNVBAcMA04vQTEgMB4GA1UECgwXU2Vs
Zi1zaWduZWQgY2VydGlmaWNhdGUxKzApBgNVBAMMIjEyMC4wLjAuMTogU2VsZi1z
aWduZWQgY2VydGlmaWNhdGUwHhcNMjEwMTI2MTYxMTM0WhcNMjEwMjI1MTYxMTM0
WjB4MQswCQYDVQQGEwJYWDEMMAoGA1UECAwDTi9BMQwwCgYDVQQHDANOL0ExIDAe
BgNVBAoMF1NlbGYtc2lnbmVkIGNlcnRpZmljYXRlMSswKQYDVQQDDCIxMjAuMC4w
LjE6IFNlbGYtc2lnbmVkIGNlcnRpZmljYXRlMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEA1WCSsgfYw2psuYAHUZXcqnRXxKEJlqlRVbDXGFapGy0moHMK
IttafGFOEzJAv765Df4x4PZTosIuc1HoLf+dhxunKWmgFtwFcAPuPwsIYuGv3cFB
gTvNBzLzEPUcB9iEi2tV9K1j8VU/pVTUF8X6xFPCcL/NHA8KSVHMsJhlroojwGDA
GS50I6d4wEv621QVeJFgDpYGLLSoPV416bLBfi2kRlPRd1jJT7Wl7kKTAMueK+Z0
WFcGBpF5nj6gtwUBqoCAZF4mgoEmeVSxTOcyhgiVcjcDsrbB/XsU6P1G20JwpURG
h1Oqv5SGu8gYky724nd3JPPcR5192vyvPzgwlwIDAQABoxMwETAPBgNVHREECDAG
hwR/AAABMA0GCSqGSIb3DQEBCwUAA4IBAQBZ1kXwQGlRV83H3V02CTN/P1hItlCk
n9PHGXJDiaLpqxCY2DPnly7jFouPPk/HGODVAYerrBaPMiteI9Fc+JedxgIADRsg
06YuXhn3qUEVBe5a6UJTA52zTXiOTyUHZmWxKbn5lchp1YRvdkLis59i4KmI6cQJ
a5+dDjw8n9PauYMKne/aielDlysBeQAZVRMvPsuMH/XJ5prLD1lq4Y1MEFEdOAsw
sFilmgeJ/BCDyBBlD4qDeAGhomMnMFb8Cm95Nrv/NreaDXn6gFVe/w3npVBp0ksl
Lh172El7qlPcY9yluZvoK8OK/hdUFanb49T0F5vQcJXeutntT6goJJM4
-----END CERTIFICATE-----
`)
