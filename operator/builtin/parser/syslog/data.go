package syslog

import (
	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"time"
)

type Case struct {
	Name                 string
	Config               *SyslogParserConfig
	InputRecord          interface{}
	ExpectedTimestamp    time.Time
	ExpectedRecord       interface{}
	ExpectedSeverity     entry.Severity
	ExpectedSeverityText string
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

func CreateCases(basicConfig func() *SyslogParserConfig) ([]Case, error) {
	location, err := testLocations()
	if err != nil {
		return nil, err
	}

	var cases = []Case{
		{
			"RFC3164",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				cfg.Location = location["utc"].String()
				return cfg
			}(),
			"<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["utc"]),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC3164Detroit",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				cfg.Location = location["detroit"].String()
				return cfg
			}(),
			"<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["detroit"]),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC3164Athens",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				cfg.Location = location["athens"].String()
				return cfg
			}(),
			"<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message",
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, location["athens"]),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC3164Bytes",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc3164"
				return cfg
			}(),
			[]byte("<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message"),
			time.Date(time.Now().Year(), 1, 12, 6, 30, 0, 0, time.UTC),
			map[string]interface{}{
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			entry.Critical,
			"crit",
		},
		{
			"RFC5424",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc5424"
				return cfg
			}(),
			`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
			time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
			map[string]interface{}{
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
			entry.Info,
			"info",
		},
		{
			"RFC5424LongSDName",
			func() *SyslogParserConfig {
				cfg := basicConfig()
				cfg.Protocol = "rfc5424"
				return cfg
			}(),
			`<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [verylongsdnamethatisgreaterthan32bytes@12345 UserHostAddress="192.168.2.132"] my message`,
			time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
			map[string]interface{}{
				"appname":  "SecureAuth0",
				"facility": 10,
				"hostname": "192.168.2.132",
				"message":  "my message",
				"msg_id":   "ID52020",
				"priority": 86,
				"proc_id":  "23108",
				"structured_data": map[string]map[string]string{
					"verylongsdnamethatisgreaterthan32bytes@12345": {
						"UserHostAddress": "192.168.2.132",
					},
				},
				"version": 1,
			},
			entry.Info,
			"info",
		},
	}

	return cases, nil

}
