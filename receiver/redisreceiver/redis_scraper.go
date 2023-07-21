// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisScraper struct {
	client   client
	redisSvc *redisSvc
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
	uptime   time.Duration
}

const redisMaxDbs = 16 // Maximum possible number of redis databases

func newRedisScraper(cfg *Config, settings receiver.CreateSettings) (scraperhelper.Scraper, error) {
	opts := &redis.Options{
		Addr:     cfg.Endpoint,
		Username: cfg.Username,
		Password: string(cfg.Password),
		Network:  cfg.Transport,
	}

	var err error
	if opts.TLSConfig, err = cfg.TLS.LoadTLSConfig(); err != nil {
		return nil, err
	}
	return newRedisScraperWithClient(newRedisClient(opts), settings, cfg)
}

func newRedisScraperWithClient(client client, settings receiver.CreateSettings, cfg *Config) (scraperhelper.Scraper, error) {
	rs := &redisScraper{
		client:   client,
		redisSvc: newRedisSvc(client),
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
	return scraperhelper.NewScraper(
		metadata.Type,
		rs.Scrape,
		scraperhelper.WithShutdown(rs.shutdown),
	)
}

func (rs *redisScraper) shutdown(context.Context) error {
	if rs.client != nil {
		return rs.client.close()
	}
	return nil
}

// Scrape is called periodically, querying Redis and building Metrics to send to
// the next consumer. First builds 'fixed' metrics (non-keyspace metrics)
// defined at startup time. Then builds 'keyspace' metrics if there are any
// keyspace lines returned by Redis. There should be one keyspace line per
// active Redis database, of which there can be 16.
func (rs *redisScraper) Scrape(context.Context) (pmetric.Metrics, error) {
	inf, err := rs.redisSvc.info()
	if err != nil {
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	currentUptime, err := inf.getUptimeInSeconds()
	if err != nil {
		return pmetric.Metrics{}, err
	}

	if rs.uptime == time.Duration(0) || rs.uptime > currentUptime {
		rs.mb.Reset(metadata.WithStartTime(pcommon.NewTimestampFromTime(now.AsTime().Add(-currentUptime))))
	}
	rs.uptime = currentUptime

	rs.recordCommonMetrics(now, inf)
	rs.recordKeyspaceMetrics(now, inf)
	rs.recordRoleMetrics(now, inf)
	rs.recordCmdStatsMetrics(now, inf)
	return rs.mb.Emit(metadata.WithRedisVersion(rs.getRedisVersion(inf))), nil
}

// recordCommonMetrics records metrics from Redis info key-value pairs.
func (rs *redisScraper) recordCommonMetrics(ts pcommon.Timestamp, inf info) {
	recorders := rs.dataPointRecorders()
	for infoKey, infoVal := range inf {
		recorder, ok := recorders[infoKey]
		if !ok {
			// Skip unregistered metric.
			continue
		}
		switch recordDataPoint := recorder.(type) {
		case func(pcommon.Timestamp, int64):
			val, err := strconv.ParseInt(infoVal, 10, 64)
			if err != nil {
				rs.settings.Logger.Warn("failed to parse info int val", zap.String("key", infoKey),
					zap.String("val", infoVal), zap.Error(err))
			}
			recordDataPoint(ts, val)
		case func(pcommon.Timestamp, float64):
			val, err := strconv.ParseFloat(infoVal, 64)
			if err != nil {
				rs.settings.Logger.Warn("failed to parse info float val", zap.String("key", infoKey),
					zap.String("val", infoVal), zap.Error(err))
			}
			recordDataPoint(ts, val)
		}
	}
}

// recordKeyspaceMetrics records metrics from 'keyspace' Redis info key-value pairs,
// e.g. "db0: keys=1,expires=2,avg_ttl=3".
func (rs *redisScraper) recordKeyspaceMetrics(ts pcommon.Timestamp, inf info) {
	for db := 0; db < redisMaxDbs; db++ {
		key := "db" + strconv.Itoa(db)
		str, ok := inf[key]
		if !ok {
			break
		}
		keyspace, parsingError := parseKeyspaceString(db, str)
		if parsingError != nil {
			rs.settings.Logger.Warn("failed to parse keyspace string", zap.String("key", key),
				zap.String("val", str), zap.Error(parsingError))
			continue
		}
		rs.mb.RecordRedisDbKeysDataPoint(ts, int64(keyspace.keys), keyspace.db)
		rs.mb.RecordRedisDbExpiresDataPoint(ts, int64(keyspace.expires), keyspace.db)
		rs.mb.RecordRedisDbAvgTTLDataPoint(ts, int64(keyspace.avgTTL), keyspace.db)
	}
}

// getRedisVersion retrieves version string from 'redis_version' Redis info key-value pairs
// e.g. "redis_version:5.0.7"
func (rs *redisScraper) getRedisVersion(inf info) string {
	if str, ok := inf["redis_version"]; ok {
		return str
	}
	return "unknown"
}

// recordRoleMetrics records metrics from 'role' Redis info key-value pairs
// e.g. "role:master"
func (rs *redisScraper) recordRoleMetrics(ts pcommon.Timestamp, inf info) {
	if str, ok := inf["role"]; ok {
		if str == "master" {
			rs.mb.RecordRedisRoleDataPoint(ts, 1, metadata.AttributeRolePrimary)
		} else {
			rs.mb.RecordRedisRoleDataPoint(ts, 1, metadata.AttributeRoleReplica)
		}
	}
}

// recordCmdStatsMetrics records metrics from 'command_stats' Redis info key-value pairs
// e.g. "cmdstat_mget:calls=1685,usec=6032,usec_per_call=3.58,rejected_calls=0,failed_calls=0"
// but only calls and usec at the moment.
func (rs *redisScraper) recordCmdStatsMetrics(ts pcommon.Timestamp, inf info) {
	cmdPrefix := "cmdstat_"
	for key, val := range inf {
		if !strings.HasPrefix(key, cmdPrefix) {
			continue
		}

		cmd := key[len(cmdPrefix):]
		parts := strings.Split(strings.TrimSpace(val), ",")
		for _, element := range parts {
			subParts := strings.Split(element, "=")
			if len(subParts) == 1 {
				continue
			}
			parsed, err := strconv.ParseInt(subParts[1], 10, 64)
			if err != nil { // skip bad items
				continue
			}
			if subParts[0] == "calls" {
				rs.mb.RecordRedisCmdCallsDataPoint(ts, parsed, cmd)
			} else if subParts[0] == "usec" {
				rs.mb.RecordRedisCmdUsecDataPoint(ts, parsed, cmd)
			}
		}
	}
}
