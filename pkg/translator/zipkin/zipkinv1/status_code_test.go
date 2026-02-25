// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv1

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestAttribToStatusCode(t *testing.T) {
	_, atoiError := strconv.ParseInt("nan", 10, 64)

	tests := []struct {
		name string
		attr pcommon.Value
		code int32
		err  error
	}{
		{
			name: "nil",
			attr: pcommon.NewValueEmpty(),
			code: 0,
			err:  errors.New("nil attribute"),
		},

		{
			name: "valid-int-code",
			attr: pcommon.NewValueInt(0),
			code: 0,
		},

		{
			name: "invalid-int-code",
			attr: pcommon.NewValueInt(int64(1 << 32)),
			code: 0,
			err:  errors.New("outside of the int32 range"),
		},

		{
			name: "valid-string-code",
			attr: pcommon.NewValueStr("200"),
			code: 200,
		},

		{
			name: "invalid-string-code",
			attr: pcommon.NewValueStr("nan"),
			code: 0,
			err:  atoiError,
		},

		{
			name: "bool-code",
			attr: pcommon.NewValueBool(true),
			code: 0,
			err:  errors.New("invalid attribute type"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := attribToStatusCode(test.attr)
			assert.Equal(t, test.code, got)
			assert.Equal(t, test.err, err)
		})
	}
}

func TestStatusCodeMapperCases(t *testing.T) {
	tests := []struct {
		name       string
		expected   ptrace.Status
		attributes map[string]string
	}{
		{
			name:     "no relevant attributes",
			expected: ptrace.NewStatus(),
			attributes: map[string]string{
				"not.relevant": "data",
			},
		},

		{
			name: "http: 500",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"http.status_code": "500",
			},
		},

		{
			name:     "http: message only, nil",
			expected: ptrace.NewStatus(),
			attributes: map[string]string{
				"http.status_message": "something",
			},
		},

		{
			name: "http: 500",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("a message")
				return ret
			}(),
			attributes: map[string]string{
				"http.status_code":    "500",
				"http.status_message": "a message",
			},
		},

		{
			name: "http: 500, with error attribute",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"http.status_code": "500",
				"error":            "an error occurred",
			},
		},

		{
			name: "oc: internal",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("a description")
				return ret
			}(),
			attributes: map[string]string{
				"census.status_code":        "13",
				"census.status_description": "a description",
			},
		},

		{
			name: "oc: description and error",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("a description")
				return ret
			}(),
			attributes: map[string]string{
				"opencensus.status_description": "a description",
				"error":                         "INTERNAL",
			},
		},

		{
			name: "oc: error only",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "INTERNAL",
			},
		},

		{
			name: "oc: empty error tag",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "",
			},
		},

		{
			name:     "oc: description only, no status",
			expected: ptrace.NewStatus(),
			attributes: map[string]string{
				"opencensus.status_description": "a description",
			},
		},

		{
			name: "oc: priority over http",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				ret.SetMessage("deadline expired")
				return ret
			}(),
			attributes: map[string]string{
				"census.status_description": "deadline expired",
				"census.status_code":        "4",

				"http.status_message": "a description",
				"http.status_code":    "500",
			},
		},

		{
			name: "error: valid oc status priority over http",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "DEADLINE_EXCEEDED",

				"http.status_message": "a description",
				"http.status_code":    "500",
			},
		},

		{
			name: "error: invalid oc status uses http",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "123",

				"http.status_message": "a description",
				"http.status_code":    "500",
			},
		},

		{
			name: "error only: string description",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "a description",
			},
		},

		{
			name: "error only: true",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "true",
			},
		},

		{
			name: "error only: false",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "false",
			},
		},

		{
			name: "error only: 1",
			expected: func() ptrace.Status {
				ret := ptrace.NewStatus()
				ret.SetCode(ptrace.StatusCodeError)
				return ret
			}(),
			attributes: map[string]string{
				"error": "1",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sMapper := &statusMapper{}
			for k, v := range test.attributes {
				sMapper.fromAttribute(k, pcommon.NewValueStr(v))
			}

			spanStatus := ptrace.NewStatus()
			sMapper.status(spanStatus)
			assert.Equal(t, test.expected, spanStatus)
		})
	}
}

func TestOTStatusFromHTTPStatus(t *testing.T) {
	for httpStatus := int32(100); httpStatus < 400; httpStatus++ {
		otelStatus := statusCodeFromHTTP(httpStatus)
		assert.Equal(t, ptrace.StatusCodeUnset, otelStatus)
	}

	for httpStatus := int32(400); httpStatus <= 604; httpStatus++ {
		otelStatus := statusCodeFromHTTP(httpStatus)
		assert.Equal(t, ptrace.StatusCodeError, otelStatus)
	}
}
