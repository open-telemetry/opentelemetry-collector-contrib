// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"

import (
	"fmt"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type Case struct {
	Name   string
	Config *Config
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

func CreateCases(basicConfig func() *Config) ([]Case, error) {
	location, err := testLocations()
	if err != nil {
		return nil, err
	}

	// We need to build out the Non-Transparent-Framing body to ensure we control the Trailer byte
	nonTransparentBodyBuilder := strings.Builder{}
	nonTransparentBodyBuilder.WriteString(`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`)
	nonTransparentBodyBuilder.WriteByte(0x00)
	nonTransparentBody := nonTransparentBodyBuilder.String()

	nulFramingTrailer := NULTrailer

	ts := time.Now()

	var cases = []Case{
		{
			"RFC3164",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC3164
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
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164Detroit",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC3164
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
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC3164Athens",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC3164
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
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
			},
			true,
			true,
		},
		{
			"RFC5424",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC5424
				return cfg
			}(),
			&entry.Entry{
				Body: `<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]interface{}{
					"appname":  "SecureAuth0",
					"facility": 10,
					"hostname": "192.168.2.132",
					"message":  "Found the user for retrieving user's profile",
					"msg_id":   "ID52020",
					"priority": 86,
					"proc_id":  "23108",
					"structured_data": map[string]map[string]string{
						"SecureAuth@27389": {
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
			"RFC6587 Octet Counting",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC5424
				cfg.EnableOctetCounting = true
				return cfg
			}(),
			&entry.Entry{
				Body: `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile` +
					`215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.133 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile` +
					`215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.134 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			},
			&entry.Entry{
				Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
				Severity:     entry.Info,
				SeverityText: "info",
				Attributes: map[string]interface{}{
					"appname":  "SecureAuth0",
					"facility": 10,
					"hostname": "192.168.2.132",
					"message":  "Found the user for retrieving user's profile",
					"msg_id":   "ID52020",
					"priority": 86,
					"proc_id":  "23108",
					"structured_data": map[string]map[string]string{
						"SecureAuth@27389": {
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
			"RFC6587 Non-Transparent-framing",
			func() *Config {
				cfg := basicConfig()
				cfg.Protocol = RFC5424
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
				Attributes: map[string]interface{}{
					"appname":  "SecureAuth0",
					"facility": 10,
					"hostname": "192.168.2.132",
					"message":  "Found the user for retrieving user's profile",
					"msg_id":   "ID52020",
					"priority": 86,
					"proc_id":  "23108",
					"structured_data": map[string]map[string]string{
						"SecureAuth@27389": {
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
