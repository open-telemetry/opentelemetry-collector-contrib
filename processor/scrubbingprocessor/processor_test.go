package scrubbingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
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
	e1.Body().SetStr(body)
	return ld

}

func Test_applyMasking(t *testing.T) {
	type fields struct {
		logger *zap.Logger
		config *Config
	}
	type args struct {
		ld plog.Logs
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected plog.Logs
	}{
		{
			name: "mask everywhere",
			fields: fields{
				logger: &zap.Logger{},
				config: &Config{
					Masking: []MaskingSettings{
						{
							Regexp:      "User",
							Placeholder: "####",
						},
					},
				},
			},
			args: args{
				ld: generateLogs("18842:M 27 Jun 10:55:21.627 # User requested shutdown...",
					map[string]string{},
					map[string]string{
						"pid":       "18842",
						"role":      "M",
						"timestamp": "27 Jun 10:55:21.627",
						"level":     "#",
						"message":   "User requested shutdown...",
					},
				),
			},
			expected: generateLogs("18842:M 27 Jun 10:55:21.627 # #### requested shutdown...",
				map[string]string{},
				map[string]string{
					"pid":       "18842",
					"role":      "M",
					"timestamp": "27 Jun 10:55:21.627",
					"level":     "#",
					"message":   "#### requested shutdown...",
				},
			),
		},

		{
			name: "mask resource attribute",
			fields: fields{
				logger: &zap.Logger{},
				config: &Config{
					Masking: []MaskingSettings{
						{
							AttributeType: "resource",
							AttributeKey:  "hostname",
							Regexp:        "otlp",
							Placeholder:   "opentelemetry",
						},
					},
				},
			},
			args: args{
				ld: generateLogs("1234567890 otlp.example.com This is a sample log line",
					map[string]string{
						"hostname": "otlp.example.com",
					},
					map[string]string{
						"timestamp": "1234567890",
						"hostname":  "otlp.example.com",
						"message":   "This is a sample log line",
					},
				),
			},
			expected: generateLogs("1234567890 opentelemetry.example.com This is a sample log line",
				map[string]string{
					"hostname": "opentelemetry.example.com",
				},
				map[string]string{
					"timestamp": "1234567890",
					"hostname":  "otlp.example.com",
					"message":   "This is a sample log line",
				},
			),
		},
		{
			name: "mask record attribute",
			fields: fields{
				logger: &zap.Logger{},
				config: &Config{
					Masking: []MaskingSettings{
						{
							AttributeType: "record",
							AttributeKey:  "hostname",
							Regexp:        "otlp",
							Placeholder:   "opentelemetry",
						},
					},
				},
			},
			args: args{
				ld: generateLogs("1234567890 otlp.example.com This is a sample log line",
					map[string]string{
						"hostname": "otlp.example.com",
					},
					map[string]string{
						"timestamp": "1234567890",
						"hostname":  "otlp.example.com",
						"message":   "This is a sample log line",
					},
				),
			},
			expected: generateLogs("1234567890 opentelemetry.example.com This is a sample log line",
				map[string]string{
					"hostname": "otlp.example.com",
				},
				map[string]string{
					"timestamp": "1234567890",
					"hostname":  "opentelemetry.example.com",
					"message":   "This is a sample log line",
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &scrubbingProcessor{
				logger: tt.fields.logger,
				config: tt.fields.config,
			}
			sp.applyMasking(tt.args.ld)
			assert.EqualValues(t, tt.args.ld.ResourceLogs().At(0).Resource().Attributes().Sort(), tt.expected.ResourceLogs().At(0).Resource().Attributes().Sort())
			assert.EqualValues(t, tt.args.ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Sort(), tt.expected.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Sort())
			assert.EqualValues(t, tt.args.ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString(), tt.expected.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
		})
	}
}

func generateLogs(body string, resourceAttributes map[string]string, recordAttributes map[string]string) plog.Logs {
	ld := plog.NewLogs()
	resourceLogs := ld.ResourceLogs().AppendEmpty()
	for k, v := range resourceAttributes {
		resourceLogs.Resource().Attributes().PutStr(k, v)
	}
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecordEntry1 := scopeLogs.LogRecords().AppendEmpty()
	for k, v := range recordAttributes {
		logRecordEntry1.Attributes().PutStr(k, v)
	}
	logRecordEntry1.Body().SetStr(body)
	return ld
}
