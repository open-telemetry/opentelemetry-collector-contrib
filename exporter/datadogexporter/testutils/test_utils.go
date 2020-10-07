package testutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// DatadogServerMock mocks a Datadog backend server
func DatadogServerMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/api/v1/validate", validateAPIKeyEndpoint)
 
	srv := httptest.NewServer(handler)
 
	return srv
}
 
type validateAPIKeyResponse struct {
	Valid bool `json:"valid"`
}

func validateAPIKeyEndpoint(w http.ResponseWriter, r *http.Request) {
	res := validateAPIKeyResponse{Valid: true}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.Write(resJSON)
}
