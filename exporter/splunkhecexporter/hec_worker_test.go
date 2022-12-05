package splunkhecexporter

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: move this test to hecWorker
func Test_pushLogData_ShouldAddResponseTo400Error(t *testing.T) {
	worker := &hecWorkerWithoutAck{
		url:     &url.URL{Scheme: "http", Host: "splunk"},
		headers: map[string]string{},
	}

	responseBody := `some error occurred`

	// An HTTP client that returns status code 400 and response body responseBody.
	worker.client, _ = newTestClient(400, responseBody)
	request := &request{
		bytes:              []byte("request body"),
		compressionEnabled: false,
		localHeaders:       map[string]string{},
	}
	// Sending logs using the client.
	err := worker.Send(context.Background(), request)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	// require.True(t, consumererror.IsPermanent(err), "Expecting permanent error")
	require.Contains(t, err.Error(), "HTTP/0.0 400")
	// The returned error should contain the response body responseBody.
	assert.Contains(t, err.Error(), responseBody)

	// An HTTP client that returns some other status code other than 400 and response body responseBody.
	worker.client, _ = newTestClient(500, responseBody)
	// Sending logs using the client.
	err = worker.Send(context.Background(), request)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	// require.False(t, consumererror.IsPermanent(err), "Expecting non-permanent error")
	require.Contains(t, err.Error(), "HTTP 500")
	// The returned error should not contain the response body responseBody.
	assert.NotContains(t, err.Error(), responseBody)
}
