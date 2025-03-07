// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap/zaptest"
)

var errMockInsert = errors.New("mock insert error")

// TestProfilesClusterConfig tests the cluster configuration.
func TestProfilesClusterConfig(t *testing.T) {
	testClusterConfig(t, func(t *testing.T, dsn string, clusterTest clusterTestConfig, fns ...func(*Config)) {
		exporter := newTestProfilesExporter(t, dsn, fns...)
		clusterTest.verifyConfig(t, exporter.cfg)
	})
}

// TestProfilesTableEngineConfig tests the table engine configuration.
func TestProfilesTableEngineConfig(t *testing.T) {
	testTableEngineConfig(t, func(t *testing.T, dsn string, engineTest tableEngineTestConfig, fns ...func(*Config)) {
		exporter := newTestProfilesExporter(t, dsn, fns...)
		engineTest.verifyConfig(t, exporter.cfg.TableEngine)
	})
}

// TestRenderProfilesSQLTemplates tests SQL template rendering with custom table names.
func TestRenderProfilesSQLTemplates(t *testing.T) {
	cfg := &Config{
		ProfilesTables: ProfilesTablesConfig{
			Profiles: "custom_profiles",
			Samples:  "custom_samples",
			Frames:   "custom_frames",
		},
	}

	// Test rendering SQL templates with custom table names
	assert.Contains(t, renderInsertProfilesSQL(cfg), "custom_profiles")
	assert.Contains(t, renderInsertSamplesSQL(cfg), "custom_samples")
	assert.Contains(t, renderInsertFramesSQL(cfg), "custom_frames")

	assert.Contains(t, renderCreateProfilesTableSQL(cfg), "custom_profiles")
	assert.Contains(t, renderCreateSamplesTableSQL(cfg), "custom_samples")
	assert.Contains(t, renderCreateFramesTableSQL(cfg), "custom_frames")
}

// TestExporter_pushProfileData tests pushing profile data to ClickHouse.
func TestExporter_pushProfileData(t *testing.T) {
	t.Run("push success", func(t *testing.T) {
		items := &atomic.Int32{}
		initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				items.Add(1)
			}
			return nil
		})
		exporter := newTestProfilesExporter(t, defaultEndpoint)
		mustPushProfileData(t, exporter, createTestProfilesData(1))

		// We expect 3 insert operations for a full profile (one for each table)
		require.Equal(t, int32(3), items.Load())
	})

	t.Run("push failure", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				return errMockInsert
			}
			return nil
		})
		exporter := newTestProfilesExporter(t, defaultEndpoint)
		err := exporter.pushProfileData(context.TODO(), createTestProfilesData(2))
		require.Error(t, err)
	})
}

// Test helpers
func newTestProfilesExporter(t *testing.T, dsn string, fns ...func(*Config)) *profilesExporter {
	exporter, err := newProfilesExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}

func mustPushProfileData(t *testing.T, exporter *profilesExporter, pd pprofile.Profiles) {
	err := exporter.pushProfileData(context.TODO(), pd)
	require.NoError(t, err)
}

// createTestProfilesData creates a minimal test profile dataset
func createTestProfilesData(count int) pprofile.Profiles {
	profiles := pprofile.NewProfiles()

	// Add a resource profile
	rp := profiles.ResourceProfiles().AppendEmpty()

	// Set resource attributes
	resource := rp.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutStr("host.name", "test-host")

	// Add a scope profile
	sp := rp.ScopeProfiles().AppendEmpty()
	scope := sp.Scope()
	scope.SetName("test-scope")
	scope.SetVersion("v1.0.0")

	for i := 0; i < count; i++ {
		// Add a profile
		profile := sp.Profiles().AppendEmpty()
		now := time.Now()
		profile.SetTime(pcommon.NewTimestampFromTime(now))
		// Duration has to be a timestamp, not a duration directly
		profile.SetDuration(pcommon.NewTimestampFromTime(now.Add(100 * time.Millisecond)))

		// Add strings to the string table - use Append to add new elements
		stringTable := profile.StringTable()
		stringTable.Append("cpu")         // Index 0
		stringTable.Append("nanoseconds") // Index 1
		stringTable.Append("samples")     // Index 2
		stringTable.Append("count")       // Index 3

		// Set period type
		periodType := profile.PeriodType()
		periodType.SetTypeStrindex(0) // "cpu" index
		periodType.SetUnitStrindex(1) // "nanoseconds" index

		// Add sample type
		sampleType := profile.SampleType().AppendEmpty()
		sampleType.SetTypeStrindex(2) // "samples" index
		sampleType.SetUnitStrindex(3) // "count" index

		// Set profile ID
		id := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, byte(i)}
		profile.SetProfileID(pprofile.ProfileID(id))

		// Add attribute to profile
		attrTable := profile.AttributeTable()
		attr := attrTable.AppendEmpty()
		attr.SetKey("test_key")
		attr.Value().SetStr("test_value")

		// Add index to profile attributes - use Append
		profile.AttributeIndices().Append(0) // Reference the first attribute

		// Add a sample
		sample := profile.Sample().AppendEmpty()

		// Add values to sample - use Append
		sample.Value().Append(int64(42))

		// Add timestamp to the sample - use Append
		sample.TimestampsUnixNano().Append(uint64(time.Now().UnixNano()))

		// Set location information
		sample.SetLocationsStartIndex(0)
		sample.SetLocationsLength(1)

		// Add attribute index to sample - use Append
		sample.AttributeIndices().Append(0) // Reference the first attribute

		// Add a location
		loc := profile.LocationTable().AppendEmpty()
		loc.SetAddress(0x1000)
		loc.SetMappingIndex(0)

		// Add line to the location
		line := loc.Line().AppendEmpty()
		line.SetFunctionIndex(0)
		line.SetLine(100)

		// Add a function
		function := profile.FunctionTable().AppendEmpty()
		function.SetNameStrindex(0)       // "cpu" index
		function.SetSystemNameStrindex(0) // "cpu" index
		function.SetFilenameStrindex(0)   // "cpu" index
		function.SetStartLine(1)

		// Add a mapping
		mapping := profile.MappingTable().AppendEmpty()
		mapping.SetFilenameStrindex(0) // "cpu" index
		mapping.SetMemoryStart(0x1000)
		mapping.SetMemoryLimit(0x2000)
	}

	return profiles
}
