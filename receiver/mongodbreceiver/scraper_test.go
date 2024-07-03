// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

func TestNewMongodbScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(receivertest.NewNopSettings(), cfg)
	require.NotEmpty(t, scraper.config.hostlist())
}

func TestScraperLifecycle(t *testing.T) {
	now := time.Now()
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newMongodbScraper(receivertest.NewNopSettings(), cfg)
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, scraper.shutdown(context.Background()))

	require.Less(t, time.Since(now), 200*time.Millisecond, "component start and stop should be very fast")
}

var (
	errAllPartialMetrics = errors.New(
		strings.Join(
			[]string{
				"failed to collect metric bytesIn: could not find key for metric",
				"failed to collect metric bytesOut: could not find key for metric",
				"failed to collect metric mongodb.asserts.msgps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.asserts.regularps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.asserts.rolloversps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.asserts.userps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.asserts.warningps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.backgroundflushing.average_ms with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.backgroundflushing.flushesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.backgroundflushing.last_ms with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.backgroundflushing.total_ms with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.cache.operations with attribute(s) miss, hit: could not find key for metric",
				"failed to collect metric mongodb.chunks.jumbo with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.chunks.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.collection.avgobjsize with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.avgobjsize with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.capped with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.capped with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.collection.indexsizes: could not find key for metric",
				"failed to collect metric mongodb.collection.indexsizes: could not find key for metric",
				"failed to collect metric mongodb.collection.max with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.max with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.maxsize with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.maxsize with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.nindexes with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.nindexes with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.objects with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.objects with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.size with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.size with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.collection.storagesize with attribute(s) fakedatabase, orders: could not find key for metric",
				"failed to collect metric mongodb.collection.storagesize with attribute(s) fakedatabase, products: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) active, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) available, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection.count with attribute(s) current, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.numascopedconnections with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.numclientconnections with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.totalavailable with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.totalcreatedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.totalinuse with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connection_pool.totalrefreshing with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.active with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.available with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.awaitingtopologychanges with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.current with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.exhausthello with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.exhaustismaster with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.loadbalanced with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.rejected with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.threaded with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.connections.totalcreated with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.cursor.count: could not find key for metric",
				"failed to collect metric mongodb.cursor.timeout.count: could not find key for metric",
				"failed to collect metric mongodb.cursors.timedout with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.cursors.totalopen with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.data.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) deleted, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) inserted, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.document.operation.count with attribute(s) updated, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.commits with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.commitsinwritelock with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.compression with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.earlycommits with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.journaledmb with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.commits with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.commitsinwritelock with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.dt with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.preplogbuffer with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.remapprivateview with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.writetodatafiles with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.timems.writetojournal with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.dur.writetodatafilesmb with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.extent.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.extra_info.heap_usage_bytesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.extra_info.page_faultsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.fsynclocked with attribute(s) admin: could not find key for metric",
				"failed to collect metric mongodb.global_lock.time: could not find key for metric",
				"failed to collect metric mongodb.globallock.activeclients.readers with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.activeclients.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.activeclients.writers with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.currentqueue.readers with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.currentqueue.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.currentqueue.writers with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.locktime with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.ratio with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.globallock.totaltime with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.health: could not find key for metric",
				"failed to collect metric mongodb.index.access.count with attribute(s) fakedatabase, orders: could not find key for index access metric",
				"failed to collect metric mongodb.index.access.count with attribute(s) fakedatabase, products: could not find key for index access metric",
				"failed to collect metric mongodb.index.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.index.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.indexcounters.accessesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.indexcounters.hitsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.indexcounters.missesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.indexcounters.missratio with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.indexcounters.resetsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirecount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirecount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirecount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirecount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirewaitcount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.acquirewaitcount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.timeacquiringmicros.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.collection.timeacquiringmicros.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirecount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirecount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirecount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirecount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirewaitcount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirewaitcount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirewaitcount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.acquirewaitcount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.timeacquiringmicros.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.timeacquiringmicros.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.timeacquiringmicros.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.database.timeacquiringmicros.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirecount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirecount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirecount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirecount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirewaitcount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirewaitcount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirewaitcount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.acquirewaitcount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.timeacquiringmicros.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.timeacquiringmicros.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.timeacquiringmicros.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.global.timeacquiringmicros.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.metadata.acquirecount.exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.metadata.acquirecount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.acquirecount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.acquirecount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.acquirewaitcount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.acquirewaitcount.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.timeacquiringmicros.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.mmapv1journal.timeacquiringmicros.intent_sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.acquirecount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.acquirecount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.acquirewaitcount.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.acquirewaitcount.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.timeacquiringmicros.intent_exclusiveps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.locks.oplog.timeacquiringmicros.sharedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.mem.bits with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.mem.mapped with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.mem.mappedwithjournal with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.mem.resident with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.mem.virtual with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.memory.usage with attribute(s) resident, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.memory.usage with attribute(s) virtual, fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.count.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.count.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.createindexes.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.createindexes.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.delete.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.delete.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.eval.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.eval.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.findandmodify.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.findandmodify.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.insert.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.insert.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.update.failedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.commands.update.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.cursor.open.notimeout with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.cursor.open.pinned with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.cursor.open.total with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.cursor.timedoutps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.document.deletedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.document.insertedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.document.returnedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.document.updatedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.getlasterror.wtime.numps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.getlasterror.wtime.totalmillisps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.getlasterror.wtimeoutsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.operation.fastmodps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.operation.idhackps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.operation.scanandorderps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.operation.writeconflictsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.queryexecutor.scannedobjectsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.queryexecutor.scannedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.record.movesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.apply.batches.numps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.apply.batches.totalmillisps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.apply.opsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.buffer.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.buffer.maxsizebytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.buffer.sizebytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.network.bytesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.network.getmores.numps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.network.getmores.totalmillisps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.network.opsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.network.readerscreatedps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.preload.docs.numps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.preload.docs.totalmillisps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.preload.indexes.numps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.repl.preload.indexes.totalmillisps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.ttl.deleteddocumentsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.metrics.ttl.passesps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.network.bytesinps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.network.bytesoutps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.network.numrequestsps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.object.count with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.commandps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.deleteps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.getmoreps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.insertps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.queryps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcounters.updateps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.commandps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.deleteps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.getmoreps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.insertps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.queryps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.opcountersrepl.updateps with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) delete: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) getmore: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) insert: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) query: could not find key for metric",
				"failed to collect metric mongodb.operation.count with attribute(s) update: could not find key for metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) read: could not find key for metric",
				"failed to collect metric mongodb.operation.latency.time with attribute(s) write: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) command: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) delete: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) getmore: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) insert: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) query: could not find key for metric",
				"failed to collect metric mongodb.operation.repl.count with attribute(s) update: could not find key for metric",
				"failed to collect metric mongodb.operation.time: could not find key for metric",
				"failed to collect metric mongodb.oplatencies.commands.latency with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.oplatencies.reads.latency with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.oplatencies.writes.latency with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.oplog.logsizemb with attribute(s) local: could not parse value as float",
				"failed to collect metric mongodb.oplog.timediff with attribute(s) local: could not find key for metric",
				"failed to collect metric mongodb.oplog.usedsizemb with attribute(s) local: could not parse value as float",
				"failed to collect metric mongodb.session.count: could not find key for metric",
				"failed to collect metric mongodb.stats.avgobjsize with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.collections with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.datasize with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.filesize with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.indexes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.indexsize with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.numextents with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.objects with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.stats.storagesize with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.storage.size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.generic.current_allocated_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.generic.heap_size with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.aggressive_memory_decommit with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.central_cache_free_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.current_total_thread_cache_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.max_total_thread_cache_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.pageheap_free_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.pageheap_unmapped_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.spinlock_total_delay_ns with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.thread_cache_free_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.tcmalloc.tcmalloc.transfer_cache_free_bytes with attribute(s) fakedatabase: could not find key for metric",
				"failed to collect metric mongodb.uptime: could not find key for metric",
				"failed to collect metric numRequests: could not find key for metric",
				"failed to collect top stats metrics: could not find key for metric",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
				"failed to find storage engine",
			}, "; "))
	errAllClientFailedFetch = errors.New(
		strings.Join(
			[]string{
				"failed to fetch admin server status metrics: some admin server status error",
				"failed to fetch collection stats metrics: some collection stats error",
				"failed to fetch collection stats metrics: some collection stats error",
				"failed to fetch database stats metrics: some database stats error",
				"failed to fetch fsyncLockStatus metrics: some fsynclock info error",
				"failed to fetch index stats metrics: some index stats error",
				"failed to fetch index stats metrics: some index stats error",
				"failed to fetch jumbo stats metrics: some jumbo stats error",
				"failed to fetch oplog stats metrics: some replication info error",
				"failed to fetch repl set get config metrics: some replset config error",
				"failed to fetch repl set status metrics: some replset status error",
				"failed to fetch server status metrics: some server status error",
				"failed to fetch top stats metrics: some top stats error",
			}, "; "))

	errCollectionNames = errors.New(
		strings.Join(
			[]string{
				"failed to fetch admin server status metrics: some admin server status error",
				"failed to fetch collection names: some collection names error",
				"failed to fetch database stats metrics: some database stats error",
				"failed to fetch fsyncLockStatus metrics: some fsynclock info error",
				"failed to fetch jumbo stats metrics: some jumbo stats error",
				"failed to fetch oplog stats metrics: some replication info error",
				"failed to fetch repl set get config metrics: some replset config error",
				"failed to fetch repl set status metrics: some replset status error",
				"failed to fetch server status metrics: some server status error",
				"failed to fetch top stats metrics: some top stats error",
			}, "; "))
)

func TestScraperScrape(t *testing.T) {
	testCases := []struct {
		desc              string
		partialErr        bool
		setupMockClient   func(t *testing.T) *fakeClient
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
	}{
		{
			desc:       "Nil client",
			partialErr: false,
			setupMockClient: func(*testing.T) *fakeClient {
				return nil
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("no client was initialized before calling scrape"),
		},
		{
			desc:       "Failed to fetch database names",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{}, errors.New("some database names error"))
				return fc
			},
			expectedMetricGen: func(*testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("failed to fetch database names: some database names error"),
		},
		{
			desc:       "Failed to fetch collection names",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("GetReplicationInfo", mock.Anything).Return(bson.M{}, errors.New("some replication info error"))
				fc.On("GetFsyncLockInfo", mock.Anything).Return(bson.M{}, errors.New("some fsynclock info error"))
				fc.On("ReplSetStatus", mock.Anything).Return(bson.M{}, errors.New("some replset status error"))
				fc.On("ReplSetConfig", mock.Anything).Return(bson.M{}, errors.New("some replset config error"))
				fc.On("JumboStats", mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some jumbo stats error"))
				fc.On("CollectionStats", mock.Anything, mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some collection stats error"))
				fc.On("ConnPoolStats", mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some connpool stats error"))
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some server status error"))
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{}, errors.New("some admin server status error"))
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some database stats error"))
				fc.On("TopStats", mock.Anything).Return(bson.M{}, errors.New("some top stats error"))
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{}, errors.New("some collection names error"))
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "partial_scrape.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errCollectionNames,
		},
		{
			desc:       "Failed to scrape client stats",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("GetReplicationInfo", mock.Anything).Return(bson.M{}, errors.New("some replication info error"))
				fc.On("GetFsyncLockInfo", mock.Anything).Return(bson.M{}, errors.New("some fsynclock info error"))
				fc.On("ReplSetStatus", mock.Anything).Return(bson.M{}, errors.New("some replset status error"))
				fc.On("ReplSetConfig", mock.Anything).Return(bson.M{}, errors.New("some replset config error"))
				fc.On("JumboStats", mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some jumbo stats error"))
				fc.On("CollectionStats", mock.Anything, mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some collection stats error"))
				fc.On("ConnPoolStats", mock.Anything, mock.Anything).Return(bson.M{}, errors.New("some connpool stats error"))
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some server status error"))
				fc.On("ServerStatus", mock.Anything, "admin").Return(bson.M{}, errors.New("some admin server status error"))
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, errors.New("some database stats error"))
				fc.On("TopStats", mock.Anything).Return(bson.M{}, errors.New("some top stats error"))
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return([]bson.M{}, errors.New("some index stats error"))
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return([]bson.M{}, errors.New("some index stats error"))
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "partial_scrape.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errAllClientFailedFetch,
		},
		{
			desc:       "Failed to scrape with partial errors on metrics",
			partialErr: true,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				wiredTigerStorage, err := loadOnlyStorageEngineAsMap()
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				indexStats, err := loadIndexStatsAsMap("error")
				require.NoError(t, err)
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("GetReplicationInfo", mock.Anything).Return(bson.M{}, nil)
				fc.On("GetFsyncLockInfo", mock.Anything).Return(bson.M{}, nil)
				fc.On("ReplSetStatus", mock.Anything).Return(bson.M{}, nil)
				fc.On("ReplSetConfig", mock.Anything).Return(bson.M{}, nil)
				fc.On("JumboStats", mock.Anything, mock.Anything).Return(bson.M{}, nil)
				fc.On("CollectionStats", mock.Anything, mock.Anything, mock.Anything).Return(bson.M{}, nil)
				fc.On("ConnPoolStats", mock.Anything, mock.Anything).Return(bson.M{}, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(bson.M{}, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(wiredTigerStorage, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(bson.M{}, nil)
				fc.On("TopStats", mock.Anything).Return(bson.M{}, nil)
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return(indexStats, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return(indexStats, nil)
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "partial_scrape.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: errAllPartialMetrics,
		},
		{
			desc:       "Successful scrape",
			partialErr: false,
			setupMockClient: func(t *testing.T) *fakeClient {
				fc := &fakeClient{}
				adminStatus, err := loadAdminStatusAsMap()
				require.NoError(t, err)
				ss, err := loadServerStatusAsMap()
				require.NoError(t, err)
				dbStats, err := loadDBStatsAsMap()
				require.NoError(t, err)
				replicationInfo, err := loadReplicationInfoAsMap()
				require.NoError(t, err)
				fsynclockInfo, err := loadFsyncLockInfoAsMap()
				require.NoError(t, err)
				replSetStatus, err := loadReplSetStatusAsMap()
				require.NoError(t, err)
				replSetConfig, err := loadReplSetConfigAsMap()
				require.NoError(t, err)
				jumboStats, err := loadJumboStatsAsMap()
				require.NoError(t, err)
				collectionStats, err := loadCollectionStatsAsMap()
				require.NoError(t, err)
				connPoolStats, err := loadConnPoolStatsAsMap()
				require.NoError(t, err)
				topStats, err := loadTopAsMap()
				require.NoError(t, err)
				productsIndexStats, err := loadIndexStatsAsMap("products")
				require.NoError(t, err)
				ordersIndexStats, err := loadIndexStatsAsMap("orders")
				require.NoError(t, err)
				mongo40, err := version.NewVersion("4.0")
				require.NoError(t, err)
				fakeDatabaseName := "fakedatabase"
				fc.On("GetVersion", mock.Anything).Return(mongo40, nil)
				fc.On("GetReplicationInfo", mock.Anything).Return(replicationInfo, nil)
				fc.On("GetFsyncLockInfo", mock.Anything).Return(fsynclockInfo, nil)
				fc.On("ReplSetStatus", mock.Anything).Return(replSetStatus, nil)
				fc.On("ReplSetConfig", mock.Anything).Return(replSetConfig, nil)
				fc.On("JumboStats", mock.Anything, mock.Anything).Return(jumboStats, nil)
				fc.On("CollectionStats", mock.Anything, mock.Anything, mock.Anything).Return(collectionStats, nil)
				fc.On("ConnPoolStats", mock.Anything, mock.Anything).Return(connPoolStats, nil)
				fc.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{fakeDatabaseName}, nil)
				fc.On("ServerStatus", mock.Anything, fakeDatabaseName).Return(ss, nil)
				fc.On("ServerStatus", mock.Anything, "admin").Return(adminStatus, nil)
				fc.On("DBStats", mock.Anything, fakeDatabaseName).Return(dbStats, nil)
				fc.On("TopStats", mock.Anything).Return(topStats, nil)
				fc.On("ListCollectionNames", mock.Anything, fakeDatabaseName).Return([]string{"products", "orders"}, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "products").Return(productsIndexStats, nil)
				fc.On("IndexStats", mock.Anything, fakeDatabaseName, "orders").Return(ordersIndexStats, nil)
				return fc
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "scraper", "expected.yaml")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraperCfg := createDefaultConfig().(*Config)
			// Enable any metrics set to `false` by default
			scraperCfg.MetricsBuilderConfig.Metrics.MongodbOperationLatencyTime.Enabled = true
			scraperCfg.MetricsBuilderConfig.Metrics.MongodbOperationReplCount.Enabled = true
			scraperCfg.MetricsBuilderConfig.Metrics.MongodbUptime.Enabled = true
			scraperCfg.MetricsBuilderConfig.Metrics.MongodbHealth.Enabled = true

			scraper := newMongodbScraper(receivertest.NewNopSettings(), scraperCfg)

			mc := tc.setupMockClient(t)
			if mc != nil {
				scraper.client = mc
			}

			actualMetrics, err := scraper.scrape(context.Background())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				if strings.Contains(err.Error(), ";") {
					// metrics with attributes use a map and errors can be returned in random order so sorting is required.
					// The first error message would not have a leading whitespace and hence split on "; "
					actualErrs := strings.Split(err.Error(), "; ")
					sort.Strings(actualErrs)
					// The first error message would not have a leading whitespace and hence split on "; "
					expectedErrs := strings.Split(tc.expectedErr.Error(), "; ")
					sort.Strings(expectedErrs)
					require.Equal(t, actualErrs, expectedErrs)
				} else {
					require.EqualError(t, err, tc.expectedErr.Error())
				}
			}

			// if mc != nil {
			// 	mc.AssertExpectations(t)
			// }

			if tc.partialErr {
				require.True(t, scrapererror.IsPartialScrapeError(err))
			} else {
				require.False(t, scrapererror.IsPartialScrapeError(err))
			}
			expectedMetrics := tc.expectedMetricGen(t)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreResourceAttributeValue("mongodb.database.name"),
				pmetrictest.IgnoreResourceAttributeValue("database"),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreScopeMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}

func TestTopMetricsAggregation(t *testing.T) {
	mont := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	loadedTop, err := loadTop()
	require.NoError(t, err)

	mont.Run("test top stats are aggregated correctly", func(mt *mtest.T) {
		mt.AddMockResponses(loadedTop)
		driver := mt.Client
		client := mongodbClient{
			Client: driver,
			logger: zap.NewNop(),
		}
		var doc bson.M
		doc, err = client.TopStats(context.Background())
		require.NoError(t, err)

		collectionPathNames, err := digForCollectionPathNames(doc)
		require.NoError(t, err)
		require.ElementsMatch(t, collectionPathNames,
			[]string{
				"config.transactions",
				"test.admin",
				"test.orders",
				"admin.system.roles",
				"local.system.replset",
				"test.products",
				"admin.system.users",
				"admin.system.version",
				"config.system.sessions",
				"local.oplog.rs",
				"local.startup_log",
			})

		actualOperationTimeValues, err := aggregateOperationTimeValues(doc, collectionPathNames, operationsMap)
		require.NoError(t, err)

		// values are taken from testdata/top.json
		expectedInsertValues := 0 + 0 + 0 + 0 + 0 + 11302 + 0 + 1163 + 0 + 0 + 0
		expectedQueryValues := 0 + 0 + 6072 + 0 + 0 + 0 + 44 + 0 + 0 + 0 + 2791
		expectedUpdateValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 155 + 9962 + 0
		expectedRemoveValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 3750 + 0
		expectedGetmoreValues := 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0 + 0
		expectedCommandValues := 540 + 397 + 4009 + 0 + 0 + 23285 + 0 + 10993 + 0 + 10116 + 0
		require.EqualValues(t, expectedInsertValues, actualOperationTimeValues["insert"])
		require.EqualValues(t, expectedQueryValues, actualOperationTimeValues["queries"])
		require.EqualValues(t, expectedUpdateValues, actualOperationTimeValues["update"])
		require.EqualValues(t, expectedRemoveValues, actualOperationTimeValues["remove"])
		require.EqualValues(t, expectedGetmoreValues, actualOperationTimeValues["getmore"])
		require.EqualValues(t, expectedCommandValues, actualOperationTimeValues["commands"])
	})
}
