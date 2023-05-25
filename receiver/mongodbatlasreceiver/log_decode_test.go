// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func TestDecode4_2(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "4.2.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeLogs(zaptest.NewLogger(t), "4.2", zippedBuffer)
	require.NoError(t, err)

	require.Equal(t, []model.LogEntry{
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:02.541+0000",
			},
			Severity:  "I",
			Component: "NETWORK",
			Context:   "listener",
			Message:   "connection accepted from 192.168.248.5:51972 #25288 (31 connections now open)",
			Raw:       "2022-09-11T18:53:02.541+0000 I  NETWORK  [listener] connection accepted from 192.168.248.5:51972 #25288 (31 connections now open)",
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:02.541+0000",
			},
			Severity:  "I",
			Component: "NETWORK",
			Context:   "listener",
			Message:   "connection accepted from 192.168.248.5:51974 #25289 (32 connections now open)",
			Raw:       "2022-09-11T18:53:02.541+0000 I  NETWORK  [listener] connection accepted from 192.168.248.5:51974 #25289 (32 connections now open)",
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:02.563+0000",
			},
			Severity:  "I",
			Component: "NETWORK",
			Context:   "conn25289",
			Message:   `received client metadata from 192.168.248.5:51974 conn25289: { driver: { name: "mongo-go-driver", version: "v1.7.2+prerelease" }, os: { type: "linux", architecture: "amd64" }, platform: "go1.18.2", application: { name: "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } }`,
			Raw:       `2022-09-11T18:53:02.563+0000 I  NETWORK  [conn25289] received client metadata from 192.168.248.5:51974 conn25289: { driver: { name: "mongo-go-driver", version: "v1.7.2+prerelease" }, os: { type: "linux", architecture: "amd64" }, platform: "go1.18.2", application: { name: "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } }`,
		},
	}, entries)
}

func TestDecode4_2InvalidLog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "4.2_invalid_log.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeLogs(zaptest.NewLogger(t), "4.2", zippedBuffer)
	require.NoError(t, err)

	require.Equal(t, []model.LogEntry{
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:02.541+0000",
			},
			Severity:  "I",
			Component: "NETWORK",
			Context:   "listener",
			Message:   "connection accepted from 192.168.248.5:51972 #25288 (31 connections now open)",
			Raw:       "2022-09-11T18:53:02.541+0000 I  NETWORK  [listener] connection accepted from 192.168.248.5:51972 #25288 (31 connections now open)",
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:02.563+0000",
			},
			Severity:  "I",
			Component: "NETWORK",
			Context:   "conn25289",
			Message:   `received client metadata from 192.168.248.5:51974 conn25289: { driver: { name: "mongo-go-driver", version: "v1.7.2+prerelease" }, os: { type: "linux", architecture: "amd64" }, platform: "go1.18.2", application: { name: "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } }`,
			Raw:       `2022-09-11T18:53:02.563+0000 I  NETWORK  [conn25289] received client metadata from 192.168.248.5:51974 conn25289: { driver: { name: "mongo-go-driver", version: "v1.7.2+prerelease" }, os: { type: "linux", architecture: "amd64" }, platform: "go1.18.2", application: { name: "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } }`,
		},
	}, entries)
}

func TestDecode4_2NotGzip(t *testing.T) {
	entries, err := decodeLogs(zaptest.NewLogger(t), "4.2", bytes.NewBuffer([]byte("Not compressed log")))
	require.ErrorContains(t, err, "gzip: invalid header")
	require.Nil(t, entries)
}

func TestDecode5_0(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "5.0.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeLogs(zaptest.NewLogger(t), "5.0", zippedBuffer)
	require.NoError(t, err)

	require.Equal(t, []model.LogEntry{
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:14.675+00:00",
			},
			Severity:  "I",
			Component: "NETWORK",
			ID:        22944,
			Context:   "conn35107",
			Message:   "Connection ended",
			Attributes: map[string]any{
				"remote":          "192.168.248.2:52066",
				"uuid":            "d3f4641a-14ca-4a24-b5bb-7d7b391a02e7",
				"connectionId":    float64(35107),
				"connectionCount": float64(33),
			},
			Raw: `{"t":{"$date":"2022-09-11T18:53:14.675+00:00"},"s":"I",  "c":"NETWORK",  "id":22944,   "ctx":"conn35107","msg":"Connection ended","attr":{"remote":"192.168.248.2:52066","uuid":"d3f4641a-14ca-4a24-b5bb-7d7b391a02e7","connectionId":35107,"connectionCount":33}}`,
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:14.676+00:00",
			},
			Severity:  "I",
			Component: "NETWORK",
			ID:        22944,
			Context:   "conn35109",
			Message:   "Connection ended",
			Attributes: map[string]any{
				"remote":          "192.168.248.2:52070",
				"uuid":            "dcdb08ac-981d-41ea-9d6b-f85fe0475bd1",
				"connectionId":    float64(35109),
				"connectionCount": float64(32),
			},
			Raw: `{"t":{"$date":"2022-09-11T18:53:14.676+00:00"},"s":"I",  "c":"NETWORK",  "id":22944,   "ctx":"conn35109","msg":"Connection ended","attr":{"remote":"192.168.248.2:52070","uuid":"dcdb08ac-981d-41ea-9d6b-f85fe0475bd1","connectionId":35109,"connectionCount":32}}`,
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:37.727+00:00",
			},
			Severity:  "I",
			Component: "SHARDING",
			ID:        20997,
			Context:   "conn9957",
			Message:   "Refreshed RWC defaults",
			Attributes: map[string]any{
				"newDefaults": map[string]any{},
			},
			Raw: `{"t":{"$date":"2022-09-11T18:53:37.727+00:00"},"s":"I",  "c":"SHARDING", "id":20997,   "ctx":"conn9957","msg":"Refreshed RWC defaults","attr":{"newDefaults":{}}}`,
		},
	}, entries)
}

func TestDecode5_0InvalidLog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "5.0_invalid_log.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeLogs(zaptest.NewLogger(t), "5.0", zippedBuffer)
	assert.NoError(t, err)

	assert.Equal(t, []model.LogEntry{
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:14.675+00:00",
			},
			Severity:  "I",
			Component: "NETWORK",
			ID:        22944,
			Context:   "conn35107",
			Message:   "Connection ended",
			Attributes: map[string]any{
				"remote":          "192.168.248.2:52066",
				"uuid":            "d3f4641a-14ca-4a24-b5bb-7d7b391a02e7",
				"connectionId":    float64(35107),
				"connectionCount": float64(33),
			},
			Raw: `{"t":{"$date":"2022-09-11T18:53:14.675+00:00"},"s":"I",  "c":"NETWORK",  "id":22944,   "ctx":"conn35107","msg":"Connection ended","attr":{"remote":"192.168.248.2:52066","uuid":"d3f4641a-14ca-4a24-b5bb-7d7b391a02e7","connectionId":35107,"connectionCount":33}}`,
		},
		{
			Timestamp: model.LogTimestamp{
				Date: "2022-09-11T18:53:37.727+00:00",
			},
			Severity:  "I",
			Component: "SHARDING",
			ID:        20997,
			Context:   "conn9957",
			Message:   "Refreshed RWC defaults",
			Attributes: map[string]any{
				"newDefaults": map[string]any{},
			},
			Raw: `{"t":{"$date":"2022-09-11T18:53:37.727+00:00"},"s":"I",  "c":"SHARDING", "id":20997,   "ctx":"conn9957","msg":"Refreshed RWC defaults","attr":{"newDefaults":{}}}`,
		},
	}, entries)
}

func TestDecode5_0NotGzip(t *testing.T) {
	entries, err := decodeLogs(zaptest.NewLogger(t), "5.0", bytes.NewBuffer([]byte("Not compressed log")))
	require.ErrorContains(t, err, "gzip: invalid header")
	require.Nil(t, entries)
}

func strp(s string) *string {
	return &s
}

func intp(i int) *int {
	return &i
}

func boolp(b bool) *bool {
	return &b
}

func TestDecodeAudit4_2(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "4.2_audit.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeAuditJSON(zaptest.NewLogger(t), zippedBuffer)
	require.NoError(t, err)

	require.Equal(t, []model.AuditLog{
		{
			Type:      "authenticate",
			Timestamp: model.LogTimestamp{Date: "2022-09-16T01:38:20.034+0000"},
			Local:     model.Address{IP: strp("127.0.0.1"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("127.0.0.1"), Port: intp(50722)},
			Users: []model.AuditUser{
				{
					Database: "admin",
					User:     "mms-automation",
				},
			},
			Roles: []model.AuditRole{
				{
					Database: "admin",
					Role:     "clusterAdmin",
				},
				{
					Database: "admin",
					Role:     "backup",
				},
				{
					Database: "admin",
					Role:     "dbAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "restore",
				},
				{
					Database: "admin",
					Role:     "userAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "readWriteAnyDatabase",
				},
			},
			Result: 0,
			Param: map[string]any{
				"db":        "admin",
				"mechanism": "SCRAM-SHA-1",
				"user":      "mms-automation",
			},
			Raw: `{ "atype" : "authenticate", "ts" : { "$date" : "2022-09-16T01:38:20.034+0000" }, "local" : { "ip" : "127.0.0.1", "port" : 27017 }, "remote" : { "ip" : "127.0.0.1", "port" : 50722 }, "users" : [ { "user" : "mms-automation", "db" : "admin" } ], "roles" : [ { "role" : "clusterAdmin", "db" : "admin" }, { "role" : "backup", "db" : "admin" }, { "role" : "dbAdminAnyDatabase", "db" : "admin" }, { "role" : "restore", "db" : "admin" }, { "role" : "userAdminAnyDatabase", "db" : "admin" }, { "role" : "readWriteAnyDatabase", "db" : "admin" } ], "param" : { "user" : "mms-automation", "db" : "admin", "mechanism" : "SCRAM-SHA-1" }, "result" : 0 }`,
		},
		{
			Type:      "authenticate",
			Timestamp: model.LogTimestamp{Date: "2022-09-16T02:37:38.714+0000"},
			Local:     model.Address{IP: strp("192.168.248.5"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.6"), Port: intp(43714)},
			Users: []model.AuditUser{
				{
					Database: "admin",
					User:     "mms-automation",
				},
			},
			Roles: []model.AuditRole{
				{
					Database: "admin",
					Role:     "clusterAdmin",
				},
				{
					Database: "admin",
					Role:     "backup",
				},
				{
					Database: "admin",
					Role:     "dbAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "restore",
				},
				{
					Database: "admin",
					Role:     "userAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "readWriteAnyDatabase",
				},
			},
			Result: 0,
			Param: map[string]any{
				"db":        "admin",
				"mechanism": "SCRAM-SHA-1",
				"user":      "mms-automation",
			},
			Raw: `{ "atype" : "authenticate", "ts" : { "$date" : "2022-09-16T02:37:38.714+0000" }, "local" : { "ip" : "192.168.248.5", "port" : 27017 }, "remote" : { "ip" : "192.168.248.6", "port" : 43714 }, "users" : [ { "user" : "mms-automation", "db" : "admin" } ], "roles" : [ { "role" : "clusterAdmin", "db" : "admin" }, { "role" : "backup", "db" : "admin" }, { "role" : "dbAdminAnyDatabase", "db" : "admin" }, { "role" : "restore", "db" : "admin" }, { "role" : "userAdminAnyDatabase", "db" : "admin" }, { "role" : "readWriteAnyDatabase", "db" : "admin" } ], "param" : { "user" : "mms-automation", "db" : "admin", "mechanism" : "SCRAM-SHA-1" }, "result" : 0 }`,
		},
		{
			Type:      "authenticate",
			Timestamp: model.LogTimestamp{Date: "2022-09-16T02:38:20.030+0000"},
			Local:     model.Address{IP: strp("127.0.0.1"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("127.0.0.1"), Port: intp(52216)},
			Users: []model.AuditUser{
				{
					Database: "admin",
					User:     "mms-automation",
				},
			},
			Roles: []model.AuditRole{
				{
					Database: "admin",
					Role:     "clusterAdmin",
				},
				{
					Database: "admin",
					Role:     "backup",
				},
				{
					Database: "admin",
					Role:     "dbAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "restore",
				},
				{
					Database: "admin",
					Role:     "userAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "readWriteAnyDatabase",
				},
			},
			Result: 0,
			Param: map[string]any{
				"db":        "admin",
				"mechanism": "SCRAM-SHA-1",
				"user":      "mms-automation",
			},
			Raw: `{ "atype" : "authenticate", "ts" : { "$date" : "2022-09-16T02:38:20.030+0000" }, "local" : { "ip" : "127.0.0.1", "port" : 27017 }, "remote" : { "ip" : "127.0.0.1", "port" : 52216 }, "users" : [ { "user" : "mms-automation", "db" : "admin" } ], "roles" : [ { "role" : "clusterAdmin", "db" : "admin" }, { "role" : "backup", "db" : "admin" }, { "role" : "dbAdminAnyDatabase", "db" : "admin" }, { "role" : "restore", "db" : "admin" }, { "role" : "userAdminAnyDatabase", "db" : "admin" }, { "role" : "readWriteAnyDatabase", "db" : "admin" } ], "param" : { "user" : "mms-automation", "db" : "admin", "mechanism" : "SCRAM-SHA-1" }, "result" : 0 }`,
		},
	}, entries)
}

func TestDecodeAudit4_2InvalidLog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "4.2_audit_invalid_log.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeAuditJSON(zaptest.NewLogger(t), zippedBuffer)
	assert.NoError(t, err)

	require.Equal(t, []model.AuditLog{
		{
			Type:      "authenticate",
			Timestamp: model.LogTimestamp{Date: "2022-09-16T01:38:20.034+0000"},
			Local:     model.Address{IP: strp("127.0.0.1"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("127.0.0.1"), Port: intp(50722)},
			Users: []model.AuditUser{
				{
					Database: "admin",
					User:     "mms-automation",
				},
			},
			Roles: []model.AuditRole{
				{
					Database: "admin",
					Role:     "clusterAdmin",
				},
				{
					Database: "admin",
					Role:     "backup",
				},
				{
					Database: "admin",
					Role:     "dbAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "restore",
				},
				{
					Database: "admin",
					Role:     "userAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "readWriteAnyDatabase",
				},
			},
			Result: 0,
			Param: map[string]any{
				"db":        "admin",
				"mechanism": "SCRAM-SHA-1",
				"user":      "mms-automation",
			},
			Raw: `{ "atype" : "authenticate", "ts" : { "$date" : "2022-09-16T01:38:20.034+0000" }, "local" : { "ip" : "127.0.0.1", "port" : 27017 }, "remote" : { "ip" : "127.0.0.1", "port" : 50722 }, "users" : [ { "user" : "mms-automation", "db" : "admin" } ], "roles" : [ { "role" : "clusterAdmin", "db" : "admin" }, { "role" : "backup", "db" : "admin" }, { "role" : "dbAdminAnyDatabase", "db" : "admin" }, { "role" : "restore", "db" : "admin" }, { "role" : "userAdminAnyDatabase", "db" : "admin" }, { "role" : "readWriteAnyDatabase", "db" : "admin" } ], "param" : { "user" : "mms-automation", "db" : "admin", "mechanism" : "SCRAM-SHA-1" }, "result" : 0 }`,
		},
		{
			Type:      "authenticate",
			Timestamp: model.LogTimestamp{Date: "2022-09-16T02:38:20.030+0000"},
			Local:     model.Address{IP: strp("127.0.0.1"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("127.0.0.1"), Port: intp(52216)},
			Users: []model.AuditUser{
				{
					Database: "admin",
					User:     "mms-automation",
				},
			},
			Roles: []model.AuditRole{
				{
					Database: "admin",
					Role:     "clusterAdmin",
				},
				{
					Database: "admin",
					Role:     "backup",
				},
				{
					Database: "admin",
					Role:     "dbAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "restore",
				},
				{
					Database: "admin",
					Role:     "userAdminAnyDatabase",
				},
				{
					Database: "admin",
					Role:     "readWriteAnyDatabase",
				},
			},
			Result: 0,
			Param: map[string]any{
				"db":        "admin",
				"mechanism": "SCRAM-SHA-1",
				"user":      "mms-automation",
			},
			Raw: `{ "atype" : "authenticate", "ts" : { "$date" : "2022-09-16T02:38:20.030+0000" }, "local" : { "ip" : "127.0.0.1", "port" : 27017 }, "remote" : { "ip" : "127.0.0.1", "port" : 52216 }, "users" : [ { "user" : "mms-automation", "db" : "admin" } ], "roles" : [ { "role" : "clusterAdmin", "db" : "admin" }, { "role" : "backup", "db" : "admin" }, { "role" : "dbAdminAnyDatabase", "db" : "admin" }, { "role" : "restore", "db" : "admin" }, { "role" : "userAdminAnyDatabase", "db" : "admin" }, { "role" : "readWriteAnyDatabase", "db" : "admin" } ], "param" : { "user" : "mms-automation", "db" : "admin", "mechanism" : "SCRAM-SHA-1" }, "result" : 0 }`,
		},
	}, entries)
}

func TestDecodeAudit5_0(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "5.0_audit.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeAuditJSON(zaptest.NewLogger(t), zippedBuffer)
	require.NoError(t, err)

	require.Equal(t, []model.AuditLog{
		{
			Type:      "clientMetadata",
			Timestamp: model.LogTimestamp{Date: "2022-09-15T23:56:28.043+00:00"},
			ID:        &model.ID{Binary: "KXMtAMh9TOOSl9aQBW1Zkg==", Type: "04"},
			Local:     model.Address{IP: strp("192.168.248.2"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.2"), Port: intp(34736)},
			Users:     []model.AuditUser{},
			Roles:     []model.AuditRole{},
			Result:    0,
			Param: map[string]any{
				"clientMetadata": map[string]any{
					"application": map[string]any{
						"name": "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)",
					},
					"driver": map[string]any{
						"name":    "mongo-go-driver",
						"version": "v1.7.2+prerelease",
					},
					"os": map[string]any{
						"architecture": "amd64",
						"type":         "linux",
					},
					"platform": "go1.18.2",
				},
				"localEndpoint": map[string]any{
					"ip":   "192.168.248.2",
					"port": float64(27017),
				},
			},
			Raw: `{ "atype" : "clientMetadata", "ts" : { "$date" : "2022-09-15T23:56:28.043+00:00" }, "uuid" : { "$binary" : "KXMtAMh9TOOSl9aQBW1Zkg==", "$type" : "04" }, "local" : { "ip" : "192.168.248.2", "port" : 27017 }, "remote" : { "ip" : "192.168.248.2", "port" : 34736 }, "users" : [], "roles" : [], "param" : { "localEndpoint" : { "ip" : "192.168.248.2", "port" : 27017 }, "clientMetadata" : { "driver" : { "name" : "mongo-go-driver", "version" : "v1.7.2+prerelease" }, "os" : { "type" : "linux", "architecture" : "amd64" }, "platform" : "go1.18.2", "application" : { "name" : "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } } }, "result" : 0 }`,
		},
		{
			Type:      "clientMetadata",
			Timestamp: model.LogTimestamp{Date: "2022-09-15T23:56:28.055+00:00"},
			ID:        &model.ID{Binary: "pWSwWRZvR9CgNsvcDYhiwg==", Type: "04"},
			Local:     model.Address{IP: strp("192.168.248.2"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.2"), Port: intp(34740)},
			Users:     []model.AuditUser{},
			Roles:     []model.AuditRole{},
			Result:    0,
			Param: map[string]any{
				"clientMetadata": map[string]any{
					"application": map[string]any{
						"name": "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)",
					},
					"driver": map[string]any{
						"name":    "mongo-go-driver",
						"version": "v1.7.2+prerelease",
					},
					"os": map[string]any{
						"architecture": "amd64",
						"type":         "linux",
					},
					"platform": "go1.18.2",
				},
				"localEndpoint": map[string]any{
					"ip":   "192.168.248.2",
					"port": float64(27017),
				},
			},
			Raw: `{ "atype" : "clientMetadata", "ts" : { "$date" : "2022-09-15T23:56:28.055+00:00" }, "uuid" : { "$binary" : "pWSwWRZvR9CgNsvcDYhiwg==", "$type" : "04" }, "local" : { "ip" : "192.168.248.2", "port" : 27017 }, "remote" : { "ip" : "192.168.248.2", "port" : 34740 }, "users" : [], "roles" : [], "param" : { "localEndpoint" : { "ip" : "192.168.248.2", "port" : 27017 }, "clientMetadata" : { "driver" : { "name" : "mongo-go-driver", "version" : "v1.7.2+prerelease" }, "os" : { "type" : "linux", "architecture" : "amd64" }, "platform" : "go1.18.2", "application" : { "name" : "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } } }, "result" : 0 }`,
		},
		{
			Type:      "logout",
			Timestamp: model.LogTimestamp{Date: "2022-09-15T23:56:28.071+00:00"},
			ID:        &model.ID{Binary: "pWSwWRZvR9CgNsvcDYhiwg==", Type: "04"},
			Local:     model.Address{IP: strp("192.168.248.2"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.2"), Port: intp(34740)},
			Users:     []model.AuditUser{},
			Roles:     []model.AuditRole{},
			Result:    0,
			Param: map[string]any{
				"initialUsers": []any{
					map[string]any{
						"db":   "local",
						"user": "__system",
					},
				},
				"reason":       "Client has disconnected",
				"updatedUsers": []any{},
			},
			Raw: `{ "atype" : "logout", "ts" : { "$date" : "2022-09-15T23:56:28.071+00:00" }, "uuid" : { "$binary" : "pWSwWRZvR9CgNsvcDYhiwg==", "$type" : "04" }, "local" : { "ip" : "192.168.248.2", "port" : 27017 }, "remote" : { "ip" : "192.168.248.2", "port" : 34740 }, "users" : [], "roles" : [], "param" : { "reason" : "Client has disconnected", "initialUsers" : [ { "user" : "__system", "db" : "local" } ], "updatedUsers" : [] }, "result" : 0 }`,
		},
	}, entries)
}

func TestDecodeAudit5_0InvalidLog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "logs", "sample-payloads", "5.0_audit_invalid_log.log"))
	require.NoError(t, err)

	zippedBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(zippedBuffer)

	_, err = gzipWriter.Write(b)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	entries, err := decodeAuditJSON(zaptest.NewLogger(t), zippedBuffer)
	assert.NoError(t, err)

	require.Equal(t, []model.AuditLog{
		{
			Type:      "clientMetadata",
			Timestamp: model.LogTimestamp{Date: "2022-09-15T23:56:28.043+00:00"},
			ID:        &model.ID{Binary: "KXMtAMh9TOOSl9aQBW1Zkg==", Type: "04"},
			Local:     model.Address{IP: strp("192.168.248.2"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.2"), Port: intp(34736)},
			Users:     []model.AuditUser{},
			Roles:     []model.AuditRole{},
			Result:    0,
			Param: map[string]any{
				"clientMetadata": map[string]any{
					"application": map[string]any{
						"name": "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)",
					},
					"driver": map[string]any{
						"name":    "mongo-go-driver",
						"version": "v1.7.2+prerelease",
					},
					"os": map[string]any{
						"architecture": "amd64",
						"type":         "linux",
					},
					"platform": "go1.18.2",
				},
				"localEndpoint": map[string]any{
					"ip":   "192.168.248.2",
					"port": float64(27017),
				},
			},
			Raw: `{ "atype" : "clientMetadata", "ts" : { "$date" : "2022-09-15T23:56:28.043+00:00" }, "uuid" : { "$binary" : "KXMtAMh9TOOSl9aQBW1Zkg==", "$type" : "04" }, "local" : { "ip" : "192.168.248.2", "port" : 27017 }, "remote" : { "ip" : "192.168.248.2", "port" : 34736 }, "users" : [], "roles" : [], "param" : { "localEndpoint" : { "ip" : "192.168.248.2", "port" : 27017 }, "clientMetadata" : { "driver" : { "name" : "mongo-go-driver", "version" : "v1.7.2+prerelease" }, "os" : { "type" : "linux", "architecture" : "amd64" }, "platform" : "go1.18.2", "application" : { "name" : "MongoDB Automation Agent v12.3.4.7674 (git: 4c7df3ac1d15ef3269d44aa38b17376ca00147eb)" } } }, "result" : 0 }`,
		},
		{
			Type:      "logout",
			Timestamp: model.LogTimestamp{Date: "2022-09-15T23:56:28.071+00:00"},
			ID:        &model.ID{Binary: "pWSwWRZvR9CgNsvcDYhiwg==", Type: "04"},
			Local:     model.Address{IP: strp("192.168.248.2"), Port: intp(27017)},
			Remote:    model.Address{IP: strp("192.168.248.2"), Port: intp(34740)},
			Users:     []model.AuditUser{},
			Roles:     []model.AuditRole{},
			Result:    0,
			Param: map[string]any{
				"initialUsers": []any{
					map[string]any{
						"db":   "local",
						"user": "__system",
					},
				},
				"reason":       "Client has disconnected",
				"updatedUsers": []any{},
			},
			Raw: `{ "atype" : "logout", "ts" : { "$date" : "2022-09-15T23:56:28.071+00:00" }, "uuid" : { "$binary" : "pWSwWRZvR9CgNsvcDYhiwg==", "$type" : "04" }, "local" : { "ip" : "192.168.248.2", "port" : 27017 }, "remote" : { "ip" : "192.168.248.2", "port" : 34740 }, "users" : [], "roles" : [], "param" : { "reason" : "Client has disconnected", "initialUsers" : [ { "user" : "__system", "db" : "local" } ], "updatedUsers" : [] }, "result" : 0 }`,
		},
	}, entries)
}

func TestDecodeAuditNotGzip(t *testing.T) {
	entries, err := decodeAuditJSON(zaptest.NewLogger(t), bytes.NewBuffer([]byte("Not compressed log")))
	require.ErrorContains(t, err, "gzip: invalid header")
	require.Nil(t, entries)
}
