package lmexporter

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"log"
)

const (
	httpPrefix = "https://"
	platformEP = ".logicmonitor.com"
)

type accountInfo interface {
	name() string
	id() string
	token(method string, payload []byte, uri string) string
}

type lmv1Account struct {
	accountName string
	accessID    string
	accessKey   string
}

func (a lmv1Account) name() string {
	return a.accountName
}

func (a lmv1Account) id() string {
	return a.accessID
}

func (a lmv1Account) token(method string, data []byte, uri string) string {
	return GenerateLMv1Token(method, a.accessID, a.accessKey, data, uri).String()
}

type bearerAccount struct {
	accountName string
	key         string
}

func (b bearerAccount) name() string {
	return b.accountName
}

func (b bearerAccount) id() string {
	return strings.Split(b.key, ":")[0]
}

func (b bearerAccount) token(method string, data []byte, uri string) string {
	return "Bearer " + b.key
}

// type credAccount struct {
// 	accountName string
// 	collid      string
// }

// func (c credAccount) name() string {
// 	return c.accountName
// }

// func (c credAccount) id() string {
// 	return c.collid
// }

// func (c credAccount) token(method string, data []byte, uri string) string {
// 	return "Collector " + c.collid + ":" + GetCredential()
// }

type HttpClient interface {
	MakeRequest(version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error)
	GetContent(url string) (*http.Response, error)
}

//LMhttpClient http client builder for LogicMonitor
type LMhttpClient struct {
	client *http.Client
	aInfo  accountInfo
}

type APIResponse struct {
	Body          []byte
	Headers       http.Header
	StatusCode    int
	ContentLength int64
}

//NewLMHTTPClient Read API key, AccessId, AccessKey settings from env and returns HttpClient Object
//set env "LOGICMONITOR_ACCOUNT", "LOGICMONITOR_ACCESS_ID/LOGICMONITOR_ACCESS_KEY" pair or LOGICMONITOR_BEARER_KEY
func NewLMHTTPClient(apitoken, headers map[string]string, needToken bool) HttpClient {
	var aInfo accountInfo
	account := os.Getenv("LOGICMONITOR_ACCOUNT")
	if account == "" {
		log.Fatal("LOGICMONITOR_ACCOUNT environment not set")
	}

	accessID, accessKey := "", ""
	// check if api token is present in config
	// if not present, then pick bearer token from config
	if needToken {
		if apitoken != nil {
			if val, ok := apitoken["access_id"]; ok {
				accessID = val
			}
			if val, ok := apitoken["access_key"]; ok {
				accessKey = val
			}
			aInfo = lmv1Account{accountName: account, accessID: accessID, accessKey: accessKey}
		} else if bearerAPI, ok := headers["Authorization"]; ok {
			aInfo = bearerAccount{accountName: account, key: strings.Split(bearerAPI, " ")[1]}
		} else {
			log.Fatal("Kindly provide Access ID/Access Key pair or Bearer token in configuration")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	httpClient := &http.Client{Transport: clientTransport, Timeout: 0}
	return &LMhttpClient{httpClient, aInfo}
}

//MakeRequest over LMHTTPClient
func (c *LMhttpClient) MakeRequest(version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error) {
	if c.client == nil {
		log.Fatal("Session not established")
	}
	var req *http.Request
	var err error
	var fullURL string
	var account string

	if c.aInfo == nil {
		account = os.Getenv("LOGICMONITOR_ACCOUNT")
	} else {
		account = c.aInfo.name()
	}
	if configURL != "" {
		fullURL = configURL + uri
	} else {
		fullURL = httpPrefix + account + platformEP + baseURI + uri
	}

	if pBytes == nil {
		pBytes = bytes.NewBuffer(nil)
	}
	req, err = http.NewRequest(method, fullURL, pBytes)
	if err != nil {
		return nil, fmt.Errorf("creation of request failed with error %w", err)
	}

	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()
	req = req.WithContext(ctx)
	req.Header.Set("X-version", version)
	req.Header.Set("x-logicmonitor-account", account)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.aInfo != nil {
		if method == http.MethodPost && pBytes != nil {
			req.Header.Set("Authorization", c.aInfo.token(method, pBytes.Bytes(), uri))
		} else {
			req.Header.Set("Authorization", c.aInfo.token(method, nil, uri))
		}
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to %s failed with error %w", fullURL, err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response %s, failed with error %w", fullURL, err)
	}
	apiResp := APIResponse{body, resp.Header, resp.StatusCode, resp.ContentLength}
	return &apiResp, nil
}

// GetContent downloads the content from specified url
func (c *LMhttpClient) GetContent(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}

// Lmv1Token defines configuration for http forwarder extension.
type Lmv1Token struct {
	AccessID  string
	Signature string
	Epoch     time.Time
}

func (t *Lmv1Token) String() string {
	builder := strings.Builder{}
	append := func(s string) {
		if _, err := builder.WriteString(s); err != nil {
			panic(err)
		}
	}
	append("LMv1 ")
	append(t.AccessID)
	append(":")
	append(t.Signature)
	append(":")
	append(strconv.FormatInt(t.Epoch.UnixNano()/1000000, 10))

	return builder.String()
}

//GenerateLMv1Token generates LMv1 Token
func GenerateLMv1Token(method string, accessID string, accessKey string, body []byte, resourcePath string) *Lmv1Token {

	epochTime := time.Now()
	epoch := strconv.FormatInt(epochTime.UnixNano()/1000000, 10)

	methodUpper := strings.ToUpper(method)

	h := hmac.New(sha256.New, []byte(accessKey))

	writeOrPanic := func(bs []byte) {
		if _, err := h.Write(bs); err != nil {
			panic(err)
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
