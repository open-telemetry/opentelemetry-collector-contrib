#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


redis-server /etc/redis-cluster-6379.conf && \
    redis-server /etc/redis-cluster-6380.conf && \
    redis-server /etc/redis-cluster-6381.conf && \
    redis-server /etc/redis-cluster-6382.conf && \
    redis-server /etc/redis-cluster-6383.conf && \
    redis-server /etc/redis-cluster-6384.conf
sleep 60
redis-cli -p 6379 ping
redis-cli -p 6380 ping
redis-cli -p 6381 ping
redis-cli -p 6382 ping
redis-cli -p 6383 ping
redis-cli -p 6384 ping

redis-cli --cluster create localhost:6379 localhost:6380 localhost:6381 localhost:6382 localhost:6383 localhost:6384 --cluster-replicas 1 --cluster-yes
