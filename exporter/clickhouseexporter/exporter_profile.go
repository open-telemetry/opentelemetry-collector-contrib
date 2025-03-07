// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // For register database driver.
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type profilesExporter struct {
	client           *sql.DB
	insertProfileSQL string
	insertSampleSQL  string
	insertFrameSQL   string
	logger           *zap.Logger
	cfg              *Config
}

func newProfilesExporter(logger *zap.Logger, cfg *Config) (*profilesExporter, error) {
	client, err := newClickhouseClient(cfg)
	if err != nil {
		return nil, err
	}

	return &profilesExporter{
		client:           client,
		insertProfileSQL: renderInsertProfilesSQL(cfg),
		insertSampleSQL:  renderInsertSamplesSQL(cfg),
		insertFrameSQL:   renderInsertFramesSQL(cfg),
		logger:           logger,
		cfg:              cfg,
	}, nil
}

func (e *profilesExporter) start(ctx context.Context, _ component.Host) error {
	if !e.cfg.shouldCreateSchema() {
		return nil
	}

	if err := createDatabase(ctx, e.cfg); err != nil {
		return err
	}

	return createProfileTables(ctx, e.cfg, e.client)
}

// shutdown will shut down the exporter.
func (e *profilesExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *profilesExporter) pushProfileData(ctx context.Context, pd pprofile.Profiles) error {
	start := time.Now()
	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		// Instead of using prepared statements for each table, use simple ExecContext
		// This avoids protocol issues that can happen with prepared statements in some ClickHouse versions

		for i := 0; i < pd.ResourceProfiles().Len(); i++ {
			resourceProfiles := pd.ResourceProfiles().At(i)
			resource := resourceProfiles.Resource()
			resAttr := internal.AttributesToMap(resource.Attributes())
			serviceName := internal.GetServiceName(resource.Attributes())
			schemaURL := resourceProfiles.SchemaUrl()

			for j := 0; j < resourceProfiles.ScopeProfiles().Len(); j++ {
				scopeProfiles := resourceProfiles.ScopeProfiles().At(j)
				scope := scopeProfiles.Scope()
				scopeSchemaURL := scopeProfiles.SchemaUrl()

				for k := 0; k < scopeProfiles.Profiles().Len(); k++ {
					profile := scopeProfiles.Profiles().At(k)
					profileID := profile.ProfileID().String()
					timeNanos := profile.Time().AsTime().UnixNano()
					durationNanos := profile.Duration().AsTime().UnixNano()
					stringTable := profile.StringTable()
					// Get period type information
					var periodTypeName, periodTypeUnit string
					var periodTypeAggregationTemporality int32
					periodTypeName = getString(stringTable, int(profile.PeriodType().TypeStrindex()))
					periodTypeUnit = getString(stringTable, int(profile.PeriodType().UnitStrindex()))

					periodTypeAggregationTemporality = int32(profile.PeriodType().AggregationTemporality())

					defaultSampleType := getString(stringTable, int(profile.DefaultSampleTypeStrindex()))

					// Get comments
					var comments []string
					commentStrindices := profile.CommentStrindices()
					for c := 0; c < commentStrindices.Len(); c++ {
						comments = append(comments, getString(stringTable, int(commentStrindices.At(c))))
					}

					// Original payload handling
					originalPayloadFormat := profile.OriginalPayloadFormat()
					originalPayload := ""
					if profile.OriginalPayload().Len() > 0 {
						originalPayload = string(profile.OriginalPayload().AsRaw())
					}

					// Insert profile record
					_, err := tx.ExecContext(ctx, e.insertProfileSQL,
						time.Unix(0, timeNanos),
						profileID,
						serviceName,
						scope.Name(),
						scope.Version(),
						resAttr,
						internal.AttributesToMap(scope.Attributes()),
						schemaURL,
						scopeSchemaURL,
						durationNanos,
						periodTypeName,
						periodTypeUnit,
						periodTypeAggregationTemporality,
						profile.Period(),
						defaultSampleType,
						comments,
						profile.DroppedAttributesCount(),
						originalPayloadFormat,
						originalPayload,
						internal.AttributesToMap(convertAttributeIndicesToMap(profile)),
					)
					if err != nil {
						return fmt.Errorf("ExecContext for profile: %w", err)
					}

					// Process sample types

					sampleType := profile.SampleType().At(0)
					sampleTypeName := getString(stringTable, int(sampleType.TypeStrindex()))
					sampleTypeUnit := getString(stringTable, int(sampleType.UnitStrindex()))
					aggregationTemporality := int(sampleType.AggregationTemporality())

					// Process samples
					for s := 0; s < profile.Sample().Len(); s++ {
						sample := profile.Sample().At(s)

						// Get trace and span ID if available
						traceID := ""
						spanID := ""
						if sample.HasLinkIndex() && int(sample.LinkIndex()) < profile.LinkTable().Len() {
							link := profile.LinkTable().At(int(sample.LinkIndex()))
							traceID = traceutil.TraceIDToHexOrEmptyString(link.TraceID())
							spanID = traceutil.SpanIDToHexOrEmptyString(link.SpanID())
						}

						// Collect all values and timestamps as arrays
						var values []int64
						var timestamps []time.Time

						// Get timestamps for the sample
						for t := 0; t < sample.TimestampsUnixNano().Len(); t++ {
							timestamps = append(timestamps, time.Unix(0, int64(sample.TimestampsUnixNano().At(t))))
						}

						// Get values for the sample
						for v := 0; v < sample.Value().Len(); v++ {
							values = append(values, sample.Value().At(v))
						}

						_, err = tx.ExecContext(ctx, e.insertSampleSQL,
							profileID,
							traceID,
							spanID,
							sampleTypeName,
							sampleTypeUnit,
							aggregationTemporality,
							values,
							timestamps,
							sample.LocationsStartIndex(),
							sample.LocationsLength(),
							s,
							internal.AttributesToMap(convertSampleAttributesToMap(profile, sample)),
						)
						if err != nil {
							return fmt.Errorf("ExecContext for sample: %w", err)
						}
					}

					// Process frames (combines locations, functions, and mappings)
					for l := 0; l < profile.LocationTable().Len(); l++ {
						location := profile.LocationTable().At(l)
						mappingIndex := location.MappingIndex()
						address := location.Address()
						locationAttr := internal.AttributesToMap(convertLocationAttributesToMap(profile, location))

						// Get mapping information if available
						var mappingMemoryStart, mappingMemoryLimit, mappingFileOffset uint64
						var mappingFilename string
						var mappingHasFunctions, mappingHasFilenames, mappingHasLineNumbers, mappingHasInlineFrames bool
						var mappingAttr column.IterableOrderedMap

						if mappingIndex >= 0 && int(mappingIndex) < profile.MappingTable().Len() {
							mapping := profile.MappingTable().At(int(mappingIndex))
							mappingMemoryStart = mapping.MemoryStart()
							mappingMemoryLimit = mapping.MemoryLimit()
							mappingFileOffset = mapping.FileOffset()
							mappingFilename = getString(stringTable, int(mapping.FilenameStrindex()))
							mappingHasFunctions = mapping.HasFunctions()
							mappingHasFilenames = mapping.HasFilenames()
							mappingHasLineNumbers = mapping.HasLineNumbers()
							mappingHasInlineFrames = mapping.HasInlineFrames()
							mappingAttr = internal.AttributesToMap(convertMappingAttributesToMap(profile, mapping))
						}

						// If location has no lines, create one row with empty function info
						if location.Line().Len() == 0 {
							_, err := tx.ExecContext(ctx, e.insertFrameSQL,
								profileID,
								l, // locationIndex
								mappingIndex,
								address,
								location.IsFolded(),
								locationAttr,
								-1, // lineIndex
								-1, // functionIndex
								"", // functionName
								"", // systemName
								"", // filename
								-1, // startLine
								-1, // line
								-1, // column
								mappingMemoryStart,
								mappingMemoryLimit,
								mappingFileOffset,
								mappingFilename,
								mappingHasFunctions,
								mappingHasFilenames,
								mappingHasLineNumbers,
								mappingHasInlineFrames,
								mappingAttr,
							)
							if err != nil {
								return fmt.Errorf("ExecContext for frame: %w", err)
							}
							continue
						}

						// Process each line for this location
						for lineIdx := 0; lineIdx < location.Line().Len(); lineIdx++ {
							line := location.Line().At(lineIdx)
							functionIndex := line.FunctionIndex()

							// Default values for function info
							functionName := ""
							systemName := ""
							filename := ""
							var startLine int64 = -1

							// Get function info if available
							if functionIndex >= 0 && functionIndex < int32(profile.FunctionTable().Len()) {
								function := profile.FunctionTable().At(int(functionIndex))
								functionName = getString(stringTable, int(function.NameStrindex()))
								systemName = getString(stringTable, int(function.SystemNameStrindex()))
								filename = getString(stringTable, int(function.FilenameStrindex()))
								startLine = function.StartLine()
							}

							_, err := tx.ExecContext(ctx, e.insertFrameSQL,
								profileID,
								l, // locationIndex
								mappingIndex,
								address,
								location.IsFolded(),
								locationAttr,
								lineIdx,
								functionIndex,
								functionName,
								systemName,
								filename,
								startLine,
								line.Line(),
								line.Column(),
								mappingMemoryStart,
								mappingMemoryLimit,
								mappingFileOffset,
								mappingFilename,
								mappingHasFunctions,
								mappingHasFilenames,
								mappingHasLineNumbers,
								mappingHasInlineFrames,
								mappingAttr,
							)
							if err != nil {
								return fmt.Errorf("ExecContext for frame: %w", err)
							}
						}
					}
				}
			}
		}
		return nil
	})
	duration := time.Since(start)
	e.logger.Debug("insert profiles", zap.Int("records", pd.SampleCount()),
		zap.String("cost", duration.String()))
	return err
}

func getString(stringTable pcommon.StringSlice, index int) string {
	if index < 0 || index >= stringTable.Len() {
		return ""
	}
	return stringTable.At(index)
}

func convertAttributeIndicesToMap(profile pprofile.Profile) pcommon.Map {
	attrs := pcommon.NewMap()

	for i := 0; i < profile.AttributeIndices().Len(); i++ {
		idx := int(profile.AttributeIndices().At(i))
		if idx >= profile.AttributeTable().Len() {
			continue
		}

		attr := profile.AttributeTable().At(idx)
		attrs.PutStr(attr.Key(), attr.Value().AsString())
	}

	return attrs
}

func convertSampleAttributesToMap(profile pprofile.Profile, sample pprofile.Sample) pcommon.Map {
	attrs := pcommon.NewMap()

	for i := 0; i < sample.AttributeIndices().Len(); i++ {
		idx := int(sample.AttributeIndices().At(i))
		if idx >= profile.AttributeTable().Len() {
			continue
		}

		attr := profile.AttributeTable().At(idx)
		attrs.PutStr(attr.Key(), attr.Value().AsString())
	}

	return attrs
}

func convertLocationAttributesToMap(profile pprofile.Profile, location pprofile.Location) pcommon.Map {
	attrs := pcommon.NewMap()

	for i := 0; i < location.AttributeIndices().Len(); i++ {
		idx := int(location.AttributeIndices().At(i))
		if idx >= profile.AttributeTable().Len() {
			continue
		}

		attr := profile.AttributeTable().At(idx)
		attrs.PutStr(attr.Key(), attr.Value().AsString())
	}

	return attrs
}

func convertMappingAttributesToMap(profile pprofile.Profile, mapping pprofile.Mapping) pcommon.Map {
	attrs := pcommon.NewMap()

	for i := 0; i < mapping.AttributeIndices().Len(); i++ {
		idx := int(mapping.AttributeIndices().At(i))
		if idx >= profile.AttributeTable().Len() {
			continue
		}

		attr := profile.AttributeTable().At(idx)
		attrs.PutStr(attr.Key(), attr.Value().AsString())
	}

	return attrs
}

const (
	// language=ClickHouse SQL
	createProfilesTableSQL = `
CREATE TABLE IF NOT EXISTS %s %s (
	Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
	ProfileId String CODEC(ZSTD(1)),
	ServiceName LowCardinality(String) CODEC(ZSTD(1)),
	ScopeName String CODEC(ZSTD(1)),
	ScopeVersion String CODEC(ZSTD(1)),
	ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	SchemaUrl String CODEC(ZSTD(1)),
	ScopeSchemaUrl String CODEC(ZSTD(1)),
	Duration Int64 CODEC(ZSTD(1)),
	PeriodTypeName String CODEC(ZSTD(1)),
	PeriodTypeUnit String CODEC(ZSTD(1)),
	PeriodTypeAggregationTemporality Int32 CODEC(ZSTD(1)),
	Period Int64 CODEC(ZSTD(1)),
	DefaultSampleType String CODEC(ZSTD(1)),
	Comments Array(String) CODEC(ZSTD(1)),
	DroppedAttributesCount UInt32 CODEC(ZSTD(1)),
	OriginalPayloadFormat String CODEC(ZSTD(1)),
	OriginalPayload String CODEC(ZSTD(1)),
	ProfileAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	INDEX idx_profile_id ProfileId TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_profile_attr_key mapKeys(ProfileAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_profile_attr_value mapValues(ProfileAttributes) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, toDateTime(Timestamp))
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`

	// language=ClickHouse SQL
	createSamplesTableSQL = `
CREATE TABLE IF NOT EXISTS %s %s (
	ProfileId String CODEC(ZSTD(1)),
	TraceId String CODEC(ZSTD(1)),
	SpanId String CODEC(ZSTD(1)),
	SampleType LowCardinality(String) CODEC(ZSTD(1)),
	SampleUnit LowCardinality(String) CODEC(ZSTD(1)),
	AggregationTemporality Int32 CODEC(ZSTD(1)),
	Values Array(Int64) CODEC(Delta, ZSTD(1)),
	Timestamps Array(DateTime64(9)) CODEC(Delta, ZSTD(1)),
	LocationsStartIndex Int32 CODEC(Delta, ZSTD(1)),
	LocationsLength Int32 CODEC(Delta, ZSTD(1)),
	Depth UInt8 CODEC(Delta, ZSTD(1)),
	Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	INDEX idx_profile_id ProfileId TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_attr_key mapKeys(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_attr_value mapValues(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
PARTITION BY toStartOfMonth(arrayElement(Timestamps, 1))
ORDER BY (ProfileId, SampleType)
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`

	// language=ClickHouse SQL
	createFramesTableSQL = `
CREATE TABLE IF NOT EXISTS %s %s (
	ProfileId String CODEC(ZSTD(1)),
	LocationIndex Int32 CODEC(Delta, ZSTD(1)),
	MappingIndex Int32 CODEC(Delta, ZSTD(1)),
	Address UInt64 CODEC(Delta, ZSTD(1)),
	IsFolded UInt8 CODEC(ZSTD(1)),
	LocationAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	LineIndex Int32 CODEC(Delta, ZSTD(1)),
	FunctionIndex Int32 CODEC(Delta, ZSTD(1)),
	FunctionName String CODEC(ZSTD(1)),
	SystemName String CODEC(ZSTD(1)),
	Filename String CODEC(ZSTD(1)),
	StartLine Int64 CODEC(Delta, ZSTD(1)),
	Line Int64 CODEC(Delta, ZSTD(1)),
	Column Int64 CODEC(Delta, ZSTD(1)),
	MappingMemoryStart UInt64 CODEC(Delta, ZSTD(1)),
	MappingMemoryLimit UInt64 CODEC(Delta, ZSTD(1)),
	MappingFileOffset UInt64 CODEC(Delta, ZSTD(1)),
	MappingFilename String CODEC(ZSTD(1)),
	MappingHasFunctions UInt8 CODEC(ZSTD(1)),
	MappingHasFilenames UInt8 CODEC(ZSTD(1)),
	MappingHasLineNumbers UInt8 CODEC(ZSTD(1)),
	MappingHasInlineFrames UInt8 CODEC(ZSTD(1)),
	MappingAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	INDEX idx_profile_id ProfileId TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_function_name FunctionName TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_file_name Filename TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_mapping_filename MappingFilename TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
ORDER BY (ProfileId, LocationIndex, LineIndex)
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`

	// language=ClickHouse SQL
	insertProfilesSQLTemplate = `INSERT INTO %s (
                        Timestamp,
                        ProfileId,
                        ServiceName,
                        ScopeName,
                        ScopeVersion,
                        ResourceAttributes,
                        ScopeAttributes,
                        SchemaUrl,
                        ScopeSchemaUrl,
                        Duration,
                        PeriodTypeName,
                        PeriodTypeUnit,
                        PeriodTypeAggregationTemporality,
                        Period,
                        DefaultSampleType,
                        Comments,
                        DroppedAttributesCount,
                        OriginalPayloadFormat,
                        OriginalPayload,
                        ProfileAttributes
                        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// language=ClickHouse SQL
	insertSamplesSQLTemplate = `INSERT INTO %s (
                        ProfileId,
                        TraceId,
                        SpanId,
                        SampleType,
                        SampleUnit,
                        AggregationTemporality,
                        Values,
                        Timestamps,
                        LocationsStartIndex,
                        LocationsLength,
                        Depth,
                        Attributes
                        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// language=ClickHouse SQL
	insertFramesSQLTemplate = `INSERT INTO %s (
                        ProfileId,
                        LocationIndex,
                        MappingIndex,
                        Address,
                        IsFolded,
                        LocationAttributes,
                        LineIndex,
                        FunctionIndex,
                        FunctionName,
                        SystemName,
                        Filename,
                        StartLine,
                        Line,
                        Column,
                        MappingMemoryStart,
                        MappingMemoryLimit,
                        MappingFileOffset,
                        MappingFilename,
                        MappingHasFunctions,
                        MappingHasFilenames,
                        MappingHasLineNumbers,
                        MappingHasInlineFrames,
                        MappingAttributes
                        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

func createProfileTables(ctx context.Context, cfg *Config, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, renderCreateProfilesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create profiles table sql: %w", err)
	}

	if _, err := db.ExecContext(ctx, renderCreateSamplesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create samples table sql: %w", err)
	}

	if _, err := db.ExecContext(ctx, renderCreateFramesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create frames table sql: %w", err)
	}

	return nil
}

func renderInsertProfilesSQL(cfg *Config) string {
	return fmt.Sprintf(strings.ReplaceAll(insertProfilesSQLTemplate, "'", "`"), cfg.ProfilesTables.Profiles)
}

func renderInsertSamplesSQL(cfg *Config) string {
	return fmt.Sprintf(strings.ReplaceAll(insertSamplesSQLTemplate, "'", "`"), cfg.ProfilesTables.Samples)
}

func renderInsertFramesSQL(cfg *Config) string {
	return fmt.Sprintf(strings.ReplaceAll(insertFramesSQLTemplate, "'", "`"), cfg.ProfilesTables.Frames)
}

func renderCreateProfilesTableSQL(cfg *Config) string {
	ttlExpr := generateTTLExpr(cfg.TTL, "toDate(Timestamp)")
	return fmt.Sprintf(createProfilesTableSQL, cfg.ProfilesTables.Profiles, cfg.clusterString(), cfg.tableEngineString(), ttlExpr)
}

func renderCreateSamplesTableSQL(cfg *Config) string {
	ttlExpr := generateTTLExpr(cfg.TTL, "toDate(arrayElement(Timestamps, 1))")
	return fmt.Sprintf(createSamplesTableSQL, cfg.ProfilesTables.Samples, cfg.clusterString(), cfg.tableEngineString(), ttlExpr)
}

func renderCreateFramesTableSQL(cfg *Config) string {
	// For frames table, don't add TTL as it has no timestamp column
	return fmt.Sprintf(createFramesTableSQL, cfg.ProfilesTables.Frames, cfg.clusterString(), cfg.tableEngineString(), "")
}
