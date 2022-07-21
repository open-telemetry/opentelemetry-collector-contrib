package scrubbingprocessor

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"
)

func TestApplyMasking(t *testing.T) {

	tests := []struct {
		name        string
		regexp      string
		input       string
		placeholder string
		expected    string
	}{
		{
			name:        "digit",
			input:       "my 344 id is 123456",
			regexp:      `\d+`,
			placeholder: "HIDDEN",
			expected:    "my HIDDEN id is HIDDEN",
		},
		{
			name:        "starts with",
			input:       "This is Test",
			regexp:      `^This`,
			placeholder: "HIDDEN",
			expected:    "HIDDEN is Test",
		},
		{
			name:        "direct",
			input:       "This is Test",
			regexp:      `Test`,
			placeholder: "HIDDEN",
			expected:    "This is HIDDEN",
		},
		{
			name:        "email",
			input:       "This is test@opsramp.com",
			regexp:      `[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`,
			placeholder: "HIDDEN",
			expected:    "This is HIDDEN",
		},
		{
			name:        "uncorrect email",
			input:       "This is opsramp.com",
			regexp:      `[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`,
			placeholder: "HIDDEN",
			expected:    "This is opsramp.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld := generateTestEntry(tt.input)
			processor := &scrubbingProcessor{config: &Config{

				Masking: []MaskingSettings{
					{
						Regexp:      tt.regexp,
						Placeholder: tt.placeholder,
					},
				},
			}}
			processor.applyMasking(ld)
			assert.Equal(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString(), tt.expected)
		})
	}

}

func generateTestEntry(body string) plog.Logs {
	ld := plog.NewLogs()
	rl0 := ld.ResourceLogs().AppendEmpty()
	sc := rl0.ScopeLogs().AppendEmpty()
	e1 := sc.LogRecords().AppendEmpty()
	e1.Body().SetStringVal(body)
	return ld

}
