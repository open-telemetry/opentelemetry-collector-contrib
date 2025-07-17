#!/bin/sh -eux
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


redis-server /etc/redis-cluster-6379.conf && \
    redis-server /etc/redis-cluster-6380.conf && \
    redis-server /etc/redis-cluster-6381.conf && \
    redis-server /etc/redis-cluster-6382.conf && \
    redis-server /etc/redis-cluster-6383.conf && \
    redis-server /etc/redis-cluster-6384.conf
redis-cli -p 6379 ping
redis-cli -p 6380 ping
redis-cli -p 6381 ping
redis-cli -p 6382 ping
redis-cli -p 6383 ping
redis-cli -p 6384 ping

redis-cli --cluster create localhost:6379 localhost:6380 localhost:6381 localhost:6382 localhost:6383 localhost:6384 --cluster-replicas 1 --cluster-yes
sleep 10s
while true; do
    if redis-cli -p 6379 cluster info | grep -q "cluster_state:ok" ; then
        break
    fi
    echo "awaiting for cluster to be ready"
    sleep 2
done


# ensure a consistent mapping to a replica on port 6385
REPLICA_PORT=$(redis-cli -p 6379 cluster nodes | grep 'slave' | awk '{print $2}' | cut -d':' -f2 | head -n 1)
REPLICA_PORT=${REPLICA_PORT%@*}
if [ -n "$REPLICA_PORT" ]; then
  echo "forwarding from port $REPLICA_PORT to 6385"
  #ssh -fNL "6385:127.0.0.1:$REPLICA_PORT" 127.0.0.1
  nohup socat tcp-listen:6385,fork,reuseaddr,forever,reuseaddr,keepalive,keepidle=10,keepintvl=10,keepcnt=2 tcp-connect:127.0.0.1:"$REPLICA_PORT" &
else
  echo "could not find replica port" 1>&2
  exit 1
fi

tail -f /dev/null
#ssh -fNL 6385:localhost:$REPLICA_PORT
