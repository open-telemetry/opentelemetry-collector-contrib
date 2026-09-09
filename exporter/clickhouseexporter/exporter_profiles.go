// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type profilesExporter struct {
	db        driver.Conn
	insertSQL string

	logger *zap.Logger
	cfg    *Config
}

func newProfilesExporter(logger *zap.Logger, cfg *Config) *profilesExporter {
	return &profilesExporter{
		insertSQL: renderInsertProfilesSQL(cfg),
		logger:    logger,
		cfg:       cfg,
	}
}

func (e *profilesExporter) start(ctx context.Context, _ component.Host) error {
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt, e.cfg.shouldCreateSchema())
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); err != nil {
			return err
		}

		if err := createProfilesTable(ctx, e.cfg, e.db); err != nil {
			return err
		}
	}

	return nil
}

func (e *profilesExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}

	return nil
}

func (e *profilesExporter) pushProfilesData(ctx context.Context, pd pprofile.Profiles) error {
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			e.logger.Warn("failed to close profiles batch", zap.Error(closeErr))
		}
	}(batch)

	processStart := time.Now()

	dic := pd.Dictionary()
	stringTable := dic.StringTable()

	var sampleCount int
	rps := pd.ResourceProfiles()
	for i := 0; i < rps.Len(); i++ {
		rp := rps.At(i)
		res := rp.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrMap := internal.AttributesToMap(resAttr)

		sps := rp.ScopeProfiles()
		for j := 0; j < sps.Len(); j++ {
			sp := sps.At(j)
			scope := sp.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()

			profiles := sp.Profiles()
			for k := 0; k < profiles.Len(); k++ {
				profile := profiles.At(k)

				profileID := profile.ProfileID().String()
				profileTime := profile.Time().AsTime()
				durationNano := profile.DurationNano()
				period := profile.Period()

				sampleType := stringAt(stringTable, profile.SampleType().TypeStrindex())
				sampleUnit := stringAt(stringTable, profile.SampleType().UnitStrindex())
				periodType := stringAt(stringTable, profile.PeriodType().TypeStrindex())
				periodUnit := stringAt(stringTable, profile.PeriodType().UnitStrindex())

				profileAttrs, err := pprofile.FromAttributeIndices(dic.AttributeTable(), profile, dic)
				if err != nil {
					return fmt.Errorf("failed to resolve profile attributes: %w", err)
				}
				profileAttrMap := internal.AttributesToMap(profileAttrs)

				samples := profile.Samples()
				for s := 0; s < samples.Len(); s++ {
					sample := samples.At(s)

					frames := resolveStack(dic, sample.StackIndex())
					traceID, spanID := resolveLink(dic, sample.LinkIndex())
					sampleAttrs, err := pprofile.FromAttributeIndices(dic.AttributeTable(), sample, dic)
					if err != nil {
						return fmt.Errorf("failed to resolve sample attributes: %w", err)
					}
					sampleAttrMap := internal.AttributesToMap(sampleAttrs)

					appendErr := batch.Append(
						profileTime,
						profileID,
						sampleType,
						sampleUnit,
						serviceName,
						resAttrMap,
						scopeName,
						scopeVersion,
						profileAttrMap,
						sampleAttrMap,
						frames.hash,
						frames.addresses,
						frames.functionNames,
						frames.fileNames,
						frames.lineNumbers,
						frames.mappingFileNames,
						sample.Values().AsRaw(),
						sample.TimestampsUnixNano().AsRaw(),
						durationNano,
						period,
						periodType,
						periodUnit,
						traceID,
						spanID,
					)
					if appendErr != nil {
						return fmt.Errorf("failed to append profile sample row: %w", appendErr)
					}

					sampleCount++
				}
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("profiles insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert profiles",
		zap.Int("records", sampleCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

type resolvedStack struct {
	hash             uint64
	addresses        []uint64
	functionNames    []string
	fileNames        []string
	lineNumbers      []int32
	mappingFileNames []string
}

func resolveStack(dic pprofile.ProfilesDictionary, stackIndex int32) resolvedStack {
	var rs resolvedStack
	stackTable := dic.StackTable()
	if stackIndex < 0 || int(stackIndex) >= stackTable.Len() {
		return rs
	}

	stringTable := dic.StringTable()
	locationTable := dic.LocationTable()
	functionTable := dic.FunctionTable()
	mappingTable := dic.MappingTable()
	hash := xxhash.New()

	locationIndices := stackTable.At(int(stackIndex)).LocationIndices()
	for li := 0; li < locationIndices.Len(); li++ {
		locIdx := locationIndices.At(li)
		if int(locIdx) >= locationTable.Len() {
			continue
		}

		location := locationTable.At(int(locIdx))
		address := location.Address()

		mappingFile := ""
		if int(location.MappingIndex()) < mappingTable.Len() {
			mappingFile = stringAt(stringTable, mappingTable.At(int(location.MappingIndex())).FilenameStrindex())
		}

		lines := location.Lines()
		if lines.Len() == 0 {
			rs.appendFrame(hash, address, "", "", 0, mappingFile)
			continue
		}

		for ln := 0; ln < lines.Len(); ln++ {
			line := lines.At(ln)
			functionName := ""
			fileName := ""
			if int(line.FunctionIndex()) < functionTable.Len() {
				fn := functionTable.At(int(line.FunctionIndex()))
				functionName = stringAt(stringTable, fn.NameStrindex())
				fileName = stringAt(stringTable, fn.FilenameStrindex())
			}

			rs.appendFrame(hash, address, functionName, fileName, int32(line.Line()), mappingFile)
		}
	}

	rs.hash = hash.Sum64()
	return rs
}

func (rs *resolvedStack) appendFrame(hash *xxhash.Digest, address uint64, functionName, fileName string, lineNumber int32, mappingFile string) {
	rs.addresses = append(rs.addresses, address)
	rs.functionNames = append(rs.functionNames, functionName)
	rs.fileNames = append(rs.fileNames, fileName)
	rs.lineNumbers = append(rs.lineNumbers, lineNumber)
	rs.mappingFileNames = append(rs.mappingFileNames, mappingFile)

	_, _ = hash.WriteString(functionName)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.WriteString(fileName)
	_, _ = hash.Write([]byte{0})
}

func resolveLink(dic pprofile.ProfilesDictionary, linkIndex int32) (traceID, spanID string) {
	linkTable := dic.LinkTable()

	if linkIndex <= 0 || int(linkIndex) >= linkTable.Len() {
		return "", ""
	}

	link := linkTable.At(int(linkIndex))
	return traceutil.TraceIDToHexOrEmptyString(link.TraceID()), traceutil.SpanIDToHexOrEmptyString(link.SpanID())
}

func stringAt(table pcommon.StringSlice, index int32) string {
	if index < 0 || int(index) >= table.Len() {
		return ""
	}

	return table.At(int(index))
}

func renderInsertProfilesSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.ProfilesInsert, cfg.database(), cfg.ProfilesTableName)
}

func renderCreateProfilesTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")

	return fmt.Sprintf(sqltemplates.ProfilesCreateTable,
		cfg.database(), cfg.ProfilesTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func createProfilesTable(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateProfilesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create profiles table sql: %w", err)
	}

	return nil
}
