package starlarktransform

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	testscript := "def transform(event):\n    json.decode(event)\n    return event"
	mockHTTPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testscript)
	}))

	testcases := []struct {
		name          string
		config        Config
		expected      string
		expectedErr   error
		validationErr error
	}{
		{
			name: "with file path",
			config: Config{
				Script: "./testdata/test.star",
			},
			expected: testscript,
		},
		{
			name: "with http url",
			config: Config{
				Script: mockHTTPServer.URL,
			},
			expected: testscript,
		},
		{
			name:          "empty script and code",
			config:        Config{},
			validationErr: errors.New("a value must be given for altest one, [code] or [script]"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.validate()
			require.Equal(t, tc.validationErr, err)
			if err != nil {
				return
			}

			code, err := tc.config.GetCode()
			require.Equal(t, tc.expectedErr, err)
			if err != nil {
				return
			}

			assert.Equal(t, tc.expected, code)
		})
	}

}
