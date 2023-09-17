package pytransform

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRunPythonCode(t *testing.T) {
	startLogServer(zap.NewNop())
	ep, err := getEmbeddedPython()
	require.NoError(t, err)

	testcases := []struct {
		name           string
		event          string
		code           string
		expectError    error
		expectedOutput string
	}{
		{
			name:           "update event input",
			event:          `{"name": "some_transform"}`,
			code:           `event['name'] = "pytransform"; send(event)`,
			expectedOutput: `{"name": "pytransform"}`,
			expectError:    nil,
		},
		{
			name:           "bad input",
			event:          `{"name": "some_transform"`,
			code:           `send(event)`,
			expectedOutput: "",
			expectError:    errors.New("error loading input event from env var"),
		},
		{
			name:           "timeout",
			event:          `{"name": "some_transform"}`,
			code:           `import time; time.sleep(2); send(event)`,
			expectedOutput: "",
			expectError:    errors.New("timeout running python script"),
		},
		{
			name:           "bad code",
			event:          `{"name": "some_transform"}`,
			code:           `send(event`,
			expectedOutput: "",
			expectError:    errors.New("error running python script"),
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runPythonCode(tt.code, tt.event, "testing", ep, zap.NewNop())
			if tt.expectError != nil {
				require.True(t, strings.Contains(err.Error(), tt.expectError.Error()))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedOutput, string(out))
		})
	}
}
