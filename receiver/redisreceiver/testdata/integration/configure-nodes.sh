#!/bin/bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

start_port=6379
end_port=6384
for port in $(seq $start_port $end_port); do
    cp /etc/redis-cluster.conf "/etc/redis-cluster-$port.conf"
    echo "port $port" >> "/etc/redis-cluster-$port.conf"
    echo "logfile /var/log/redis/redis-server-$port.log" >> "/etc/redis-cluster-$port.conf"
    echo "cluster-config-file nodes-$port.conf" >> "/etc/redis-cluster-$port.conf"
done
