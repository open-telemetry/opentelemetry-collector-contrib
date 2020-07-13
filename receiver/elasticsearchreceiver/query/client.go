package query

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	es "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/client"
)

type ESQueryHTTPClient struct {
	esClient *es.ESClient
}

// NewESQueryClient creates a new ESQueryHTTPClient
func NewESQueryClient(endpoint string, client *http.Client) ESQueryHTTPClient {
	return ESQueryHTTPClient{
		esClient: &es.ESClient{
			Endpoint:   endpoint,
			HTTPClient: client,
		},
	}
}

// Returns a response for a given elasticsearch query
func (es ESQueryHTTPClient) HTTPRequestFromConfig(index string, esSearchRequest string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/_search?", es.esClient.Endpoint, index)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(esSearchRequest)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := es.esClient.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
