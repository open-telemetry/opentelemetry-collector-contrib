// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog/syslogtest"

import (
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
)

// This is the name of a test which requires setting the PreserveWhitespace flags.
const RFC6587OctetCountingPreserveSpaceTest = "RFC6587 Octet Counting Preserve Space"

type Case struct {
	Name   string
	Config *syslog.Config
	Input  *entry.Entry
	Expect *entry.Entry

	// These signal if a test is valid for UDP and/or TCP protocol
	ValidForTCP bool
	ValidForUDP bool
}

func testLocations() (map[string]*time.Location, error) {
	locations := map[string]string{
		"utc":     "UTC",
		"detroit": "America/Detroit",
		"athens":  "Europe/Athens",
	}

	l := make(map[string]*time.Location)
	for k, v := range locations {
		var err error
		if l[k], err = time.LoadLocation(v); err != nil {
			return nil, err
		}
	}
	return l, nil
}

func CreateCases(basicConfig func() *syslog.Config) ([]Case, error) {
	location, err := testLocations()
	if err != nil {
		return nil, err
	}

	// We need to build out the Non-Transparent-Framing body to ensure we control the Trailer byte
	nonTransparentBodyBuilder := strings.Builder{}
	nonTransparentBodyBuilder.WriteString(`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`)
	nonTransparentBodyBuilder.WriteByte(0x00)
	nonTransparentBody := nonTransparentBodyBuilder.String()

	nulFramingTrailer := syslog.NULTrailer

	ts := time.Now()

	cases := []Case{
		{
			"RFC3164SkipPriAbsent",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC3164
				cfg.Location = location["utc"].String()
				cfg.AllowSkipPriHeader = true
				return cfg
			}(),
			&entry.Entry{
				Body: fmt.Sprintf("%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			&entry.Entry{
				Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, location["utc"]),
				Severity:     entry.Default,
				SeverityText: "",
				Attributes: map[string]any{
					"appname":  "apache_server",
					"hostname": "1.2.3.4",
					"message":  "test message",
				},
				Body: fmt.Sprintf("%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164SkipPriPresent",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC3164
				cfg.Location = location["utc"].String()
				cfg.AllowSkipPriHeader = true
				return cfg
			}(),
			&entry.Entry{
				Body: fmt.Sprintf("<123>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			&entry.Entry{
				Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, location["utc"]),
				Severity:     entry.Error,
				SeverityText: "err",
				Attributes: map[string]any{
					"appname":       "apache_server",
					"hostname":      "1.2.3.4",
					"message":       "test message",
					"facility":      15,
					"facility_text": "cron2",
					"priority":      123,
				},
				Body: fmt.Sprintf("<123>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC3164
				cfg.Location = location["utc"].String()
				return cfg
			}(),
			&entry.Entry{
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			&entry.Entry{
				Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, location["utc"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]any{
					"appname":       "apache_server",
					"facility":      4,
					"facility_text": "auth",
					"hostname":      "1.2.3.4",
					"message":       "test message",
					"priority":      34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164Detroit",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC3164
				cfg.Location = location["detroit"].String()
				return cfg
			}(),
			&entry.Entry{
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			&entry.Entry{
				Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, location["detroit"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]any{
					"appname":       "apache_server",
					"facility":      4,
					"facility_text": "auth",
					"hostname":      "1.2.3.4",
					"message":       "test message",
					"priority":      34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164Athens",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC3164
				cfg.Location = location["athens"].String()
				return cfg
			}(),
			&entry.Entry{
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			&entry.Entry{
				Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, location["athens"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]any{
					"appname":       "apache_server",
					"facility":      4,
					"facility_text": "auth",
					"hostname":      "1.2.3.4",
					"message":       "test message",
					"priority":      34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC5424",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				return cfg
			}(),
			&entry.Entry{
				Body: `<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]any{
					"appname":       "SecureAuth0",
					"facility":      10,
					"facility_text": "authpriv",
					"hostname":      "192.168.2.132",
					"message":       "Found the user for retrieving user's profile",
					"msg_id":        "ID52020",
					"priority":      86,
					"proc_id":       "23108",
					"structured_data": map[string]any{
						"SecureAuth@27389": map[string]any{
							"PEN":             "27389",
							"Realm":           "SecureAuth0",
							"UserHostAddress": "192.168.2.132",
							"UserID":          "Tester2",
						},
					},
					"version": 1,
				},
				Body: `<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			true,
			true,
		},
		{
			"RFC5424SkipPriAbsent",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				cfg.AllowSkipPriHeader = true
				return cfg
			}(),
			&entry.Entry{
				Body: `1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Default,
				SeverityText: "",
				Attributes: map[string]any{
					"appname":  "SecureAuth0",
					"hostname": "192.168.2.132",
					"message":  "Found the user for retrieving user's profile",
					"msg_id":   "ID52020",
					"proc_id":  "23108",
					"structured_data": map[string]any{
						"SecureAuth@27389": map[string]any{
							"PEN":             "27389",
							"Realm":           "SecureAuth0",
							"UserHostAddress": "192.168.2.132",
							"UserID":          "Tester2",
						},
					},
					"version": 1,
				},
				Body: `1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			true,
			true,
		},
		{
			"RFC5424SkipPriPresent",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				cfg.AllowSkipPriHeader = true
				return cfg
			}(),
			&entry.Entry{
				Body: `<123>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Error,
				SeverityText: "err",
				Attributes: map[string]any{
					"appname":  "SecureAuth0",
					"hostname": "192.168.2.132",
					"message":  "Found the user for retrieving user's profile",
					"msg_id":   "ID52020",
					"proc_id":  "23108",
					"structured_data": map[string]any{
						"SecureAuth@27389": map[string]any{
							"PEN":             "27389",
							"Realm":           "SecureAuth0",
							"UserHostAddress": "192.168.2.132",
							"UserID":          "Tester2",
						},
					},
					"version":       1,
					"facility":      15,
					"facility_text": "cron2",
					"priority":      123,
				},
				Body: `<123>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			true,
			true,
		},
		{
			"RFC6587 Octet Counting",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				cfg.EnableOctetCounting = true
				return cfg
			}(),
			&entry.Entry{
				Body: `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]any{
					"appname":       "SecureAuth0",
					"facility":      10,
					"facility_text": "authpriv",
					"hostname":      "192.168.2.132",
					"message":       "Found the user for retrieving user's profile",
					"msg_id":        "ID52020",
					"priority":      86,
					"proc_id":       "23108",
					"structured_data": map[string]any{
						"SecureAuth@27389": map[string]any{
							"PEN":             "27389",
							"Realm":           "SecureAuth0",
							"UserHostAddress": "192.168.2.132",
							"UserID":          "Tester2",
						},
					},
					"version": 1,
				},
				Body: `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			true,
			false,
		},
		{
			RFC6587OctetCountingPreserveSpaceTest,
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				cfg.EnableOctetCounting = true
				return cfg
			}(),
			&entry.Entry{
				Body: `77 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 inactive - - -  partition is p2 `,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]any{
					"appname":       "inactive",
					"facility":      10,
					"facility_text": "authpriv",
					"hostname":      "192.168.2.132",
					"message":       " partition is p2 ",
					"priority":      86,
					"version":       1,
				},
				Body: `77 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 inactive - - -  partition is p2 `,
			},
			true,
			false,
		},
		{
			"RFC6587 Non-Transparent-framing",
			func() *syslog.Config {
				cfg := basicConfig()
				cfg.Protocol = syslog.RFC5424
				cfg.NonTransparentFramingTrailer = &nulFramingTrailer
				return cfg
			}(),
			&entry.Entry{
				Body: nonTransparentBody,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]any{
					"appname":       "SecureAuth0",
					"facility":      10,
					"facility_text": "authpriv",
					"hostname":      "192.168.2.132",
					"message":       "Found the user for retrieving user's profile",
					"msg_id":        "ID52020",
					"priority":      86,
					"proc_id":       "23108",
					"structured_data": map[string]any{
						"SecureAuth@27389": map[string]any{
							"PEN":             "27389",
							"Realm":           "SecureAuth0",
							"UserHostAddress": "192.168.2.132",
							"UserID":          "Tester2",
						},
					},
					"version": 1,
				},
				Body: nonTransparentBody,
			},
			true,
			false,
		},
	}

	return cases, nil
}
