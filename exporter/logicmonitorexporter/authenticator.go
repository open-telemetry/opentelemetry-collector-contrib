// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LMAuthenticator is used for authenticating requests to Logicmonitor platform
type LMAuthenticator struct {
	Config *Config
}

// GetCredentials implements AuthProvider interface (https://github.com/logicmonitor/lm-data-sdk-go/blob/b135130a6cb411007cb4f8866966f00c94b7c63a/model/authprovider.go#L3)
func (a LMAuthenticator) GetCredentials(method, uri string, body []byte) string {
	// Fetches token from config file, if present
	// OR reads token from environment variable
	authToken := getToken(a.Config.APIToken, a.Config.Headers)
	if authToken.accessID != "" && authToken.accessKey != "" {
		return generateLMv1Token(method, authToken.accessID, authToken.accessKey, body, uri).String()
	} else if authToken.bearerToken != "" {
		return authToken.bearerToken
	}
	return ""
}

func getAccessToken(apitoken map[string]string) (string, string, bool) {
	accessID, accessKey := "", ""
	for k, v := range apitoken {
		if strings.EqualFold(k, "access_id") {
			accessID = v
		} else if strings.EqualFold(k, "access_key") {
			accessKey = v
		}
	}
	if accessID != "" && accessKey != "" {
		return accessID, accessKey, true
	}
	return "", "", false
}

func getBearerToken(headers map[string]string) (string, bool) {
	bearerToken := ""
	for k, v := range headers {
		if strings.EqualFold(k, "authorization") {
			bearerToken = strings.Split(v, " ")[1]
			return bearerToken, true
		}
	}
	return "", false
}

type AuthToken struct {
	accessID    string
	accessKey   string
	bearerToken string
}

func getToken(apiToken, headers map[string]string) AuthToken {
	accessID, accessKey, ok := getAccessToken(apiToken)
	if !ok {
		accessID = os.Getenv("LOGICMONITOR_ACCESS_ID")
		accessKey = os.Getenv("LOGICMONITOR_ACCESS_KEY")
	}
	bearerToken, ok := getBearerToken(headers)
	if !ok {
		bearerToken = os.Getenv("LOGICMONITOR_BEARER_TOKEN")
	}
	authToken := AuthToken{
		accessID:    accessID,
		accessKey:   accessKey,
		bearerToken: bearerToken,
	}
	return authToken
}

type Lmv1Token struct {
	AccessID  string
	Signature string
	Epoch     time.Time
}

func (t *Lmv1Token) String() string {
	builder := strings.Builder{}
	appendSignature := func(s string) {
		if _, err := builder.WriteString(s); err != nil {
			fmt.Println(err) //TODO: print err in place of panic
		}
	}
	appendSignature("LMv1 ")
	appendSignature(t.AccessID)
	appendSignature(":")
	appendSignature(t.Signature)
	appendSignature(":")
	appendSignature(strconv.FormatInt(t.Epoch.UnixNano()/1000000, 10))

	return builder.String()
}

// GenerateLMv1Token generates LMv1 Token
func generateLMv1Token(method string, accessID string, accessKey string, body []byte, resourcePath string) *Lmv1Token {

	epochTime := time.Now()
	epoch := strconv.FormatInt(epochTime.UnixNano()/1000000, 10)

	methodUpper := strings.ToUpper(method)

	h := hmac.New(sha256.New, []byte(accessKey))

	writeOrPanic := func(bs []byte) {
		if _, err := h.Write(bs); err != nil {
			fmt.Println(err) //TODO: print err in place of panic
		}
	}
	writeOrPanic([]byte(methodUpper))
	writeOrPanic([]byte(epoch))
	if body != nil {
		writeOrPanic(body)
	}
	writeOrPanic([]byte(resourcePath))

	hash := h.Sum(nil)
	hexString := hex.EncodeToString(hash)
	signature := base64.StdEncoding.EncodeToString([]byte(hexString))
	return &Lmv1Token{
		AccessID:  accessID,
		Signature: signature,
		Epoch:     epochTime,
	}
}
