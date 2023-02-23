// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

window.onload = function () {
  const documentBodyView = new DocumentBodyView(document);
  const processorDashboardController = new ExtensionDashboardController(
    documentBodyView,
    new StyleBundle(),
    new TestRegisteredProcessorsFetcher(),
    path => new TestWebSocketConnector(path)
  );
  processorDashboardController.fetchFromServer();
};

class TestRegisteredProcessorsFetcher {

  fetchProcessors(consumeProcessorInfo) {
    consumeProcessorInfo([
      {Name: '1', Port: 12001, Limit: 1},
      {Name: '2', Port: 12002, Limit: 1},
    ]);
  }

}

const wsResponseObject = {
  "resourceMetrics": [{
    "resource": {"attributes": [{"key": "redis.version", "value": {"stringValue": "6.2.6"}}]}, "scopeMetrics": [{
      "scope": {"name": "otelcol/redisreceiver", "version": "0.71.0-dev"},
      "metrics": [{
        "name": "redis.clients.blocked",
        "description": "Number of clients pending on a blocking call",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2
        }
      }, {
        "name": "redis.clients.connected",
        "description": "Number of client connections (excluding connections from replicas)",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "1"
          }], "aggregationTemporality": 2
        }
      }, {
        "name": "redis.clients.max_input_buffer",
        "description": "Biggest input buffer among current client connections",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.clients.max_output_buffer",
        "description": "Longest output list among current client connections",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.commands",
        "description": "Number of commands processed per second",
        "unit": "{ops}/s",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.commands.processed",
        "description": "Total number of commands processed by the server",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "247"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.connections.received",
        "description": "Total number of connections accepted by the server",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "43"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.connections.rejected",
        "description": "Number of connections rejected because of maxclients limit",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.cpu.time",
        "description": "System CPU consumed by the Redis server in seconds since server start",
        "unit": "s",
        "sum": {
          "dataPoints": [{
            "attributes": [{"key": "state", "value": {"stringValue": "user_main_thread"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 70.528791
          }, {
            "attributes": [{"key": "state", "value": {"stringValue": "user"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 70.588458
          }, {
            "attributes": [{"key": "state", "value": {"stringValue": "sys_main_thread"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 161.721824
          }, {
            "attributes": [{"key": "state", "value": {"stringValue": "sys_children"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 0.002705
          }, {
            "attributes": [{"key": "state", "value": {"stringValue": "sys"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 161.836821
          }, {
            "attributes": [{"key": "state", "value": {"stringValue": "user_children"}}],
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 0.00506
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.keys.evicted",
        "description": "Number of evicted keys due to maxmemory limit",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.keys.expired",
        "description": "Total number of key expiration events",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.keyspace.hits",
        "description": "Number of successful lookup of keys in the main dictionary",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.keyspace.misses",
        "description": "Number of failed lookup of keys in the main dictionary",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.latest_fork",
        "description": "Duration of the latest fork operation in microseconds",
        "unit": "us",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.memory.fragmentation_ratio",
        "description": "Ratio between used_memory_rss and used_memory",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asDouble": 9.24
          }]
        }
      }, {
        "name": "redis.memory.lua",
        "description": "Number of bytes used by the Lua engine",
        "unit": "By",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "37888"
          }]
        }
      }, {
        "name": "redis.memory.peak",
        "description": "Peak memory consumed by Redis (in bytes)",
        "unit": "By",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "873640"
          }]
        }
      }, {
        "name": "redis.memory.rss",
        "description": "Number of bytes that Redis allocated as seen by the operating system",
        "unit": "By",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "7487488"
          }]
        }
      }, {
        "name": "redis.memory.used",
        "description": "Total number of bytes allocated by Redis using its allocator",
        "unit": "By",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "873640"
          }]
        }
      }, {
        "name": "redis.net.input",
        "description": "The total number of bytes read from the network",
        "unit": "By",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "5704"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.net.output",
        "description": "The total number of bytes written to the network",
        "unit": "By",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "1032998"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }, {
        "name": "redis.rdb.changes_since_last_save",
        "description": "Number of changes since the last dump",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2
        }
      }, {
        "name": "redis.replication.backlog_first_byte_offset",
        "description": "The master offset of the replication backlog buffer",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.replication.offset",
        "description": "The server's current replication offset",
        "gauge": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }]
        }
      }, {
        "name": "redis.slaves.connected",
        "description": "Number of connected replicas",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "0"
          }], "aggregationTemporality": 2
        }
      }, {
        "name": "redis.uptime",
        "description": "Number of seconds since Redis server start",
        "unit": "s",
        "sum": {
          "dataPoints": [{
            "startTimeUnixNano": "1677706701310136000",
            "timeUnixNano": "1678229193310136000",
            "asInt": "522492"
          }], "aggregationTemporality": 2, "isMonotonic": true
        }
      }]
    }]
  }]
}

class TestWebSocketConnector {

  constructor(path) {
    console.log('TestWebSocketConnector', 'path', path);
    if (path == 'ws://localhost:12001') {
      this.numMessages = 1;
    } else {
      this.numMessages = 36;
    }
  }

  start(jsonConsumer) {
    for (let i = 0; i < this.numMessages; i++) {
      jsonConsumer(JSON.stringify(wsResponseObject));
    }
  }

}
