#!/bin/bash

# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Cluster start script to bootstrap a Riak cluster.
#
set -ex

if [[ -x /usr/sbin/riak ]]; then
  export RIAK=/usr/sbin/riak
else
  export RIAK=$RIAK_HOME/bin/riak
fi
export RIAK_CONF=/etc/riak/riak.conf
export USER_CONF=/etc/riak/user.conf
export RIAK_ADVANCED_CONF=/etc/riak/advanced.config
if [[ -x /usr/sbin/riak-admin ]]; then
  export RIAK_ADMIN=/usr/sbin/riak-admin
else
  export RIAK_ADMIN=$RIAK_HOME/bin/riak-admin
fi
export SCHEMAS_DIR=/etc/riak/schemas/

# Set ports for PB and HTTP
export PB_PORT=${PB_PORT:-8087}
export HTTP_PORT=${HTTP_PORT:-8098}

# Use ping to discover our HOSTNAME because it's easier and more reliable than other methods
export HOST=$(ping -c1 $HOSTNAME | awk '/^PING/ {print $3}' | sed 's/[()]//g')||'127.0.0.1'

# CLUSTER_NAME is used to name the nodes and is the value used in the distributed cookie
export CLUSTER_NAME=${CLUSTER_NAME:-riak}

# The COORDINATOR_NODE is the first node in a cluster to which other nodes will eventually join
export COORDINATOR_NODE=${COORDINATOR_NODE:-$HOSTNAME}
export COORDINATOR_NODE_HOST=$(ping -c1 $COORDINATOR_NODE | awk '/^PING/ {print $3}' | sed 's/[()]//g')||'127.0.0.1'

# Start the node and wait until fully up
$RIAK start

# Trap SIGTERM and SIGINT and tail the log file indefinitely
tail -n 1024 -f /var/log/riak/console.log

