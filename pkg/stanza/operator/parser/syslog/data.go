// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type Case struct {
	Name   string
	Config *Config
	Input  *entry.Entry
	Expect *entry.Entry
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
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
			&entry.Entry{
				Timestamp:    time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["utc"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
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
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
			&entry.Entry{
				Timestamp:    time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["detroit"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
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
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
			&entry.Entry{
				Timestamp:    time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["athens"]),
				Severity:     entry.Error2,
				SeverityText: "crit",
				Attributes: map[string]interface{}{
					"appname":  "apache_server",
					"facility": 4,
					"hostname": "1.2.3.4",
					"message":  "test message",
					"priority": 34,
				},
				Body: "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			},
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
		},
	}

	return cases, nil
}
