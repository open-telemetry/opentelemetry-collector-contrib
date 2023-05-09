#!/usr/bin/env bash

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

set -e

USER='otelu'
ROOT_PASS='otelp'

# Test if rabbitmqctl is working at all
rabbitmqctl list_users > /dev/null

# Don't recreate user if already exists
if ! rabbitmqctl list_users | grep otelu > /dev/null; then
    echo "creating user: ${USER} . . ."
    rabbitmqctl add_user ${USER} ${ROOT_PASS} > /dev/null

    echo "Configuring ${USER} permissions. . ."
    rabbitmqctl set_user_tags ${USER} administrator > /dev/null
    rabbitmqctl set_permissions -p /  ${USER} ".*" ".*" ".*" > /dev/null
fi

echo "create exchange and queue. . ."
rabbitmqadmin -u ${USER} -p ${ROOT_PASS} declare exchange --vhost=/ name=some_exchange type=direct > /dev/null
rabbitmqadmin -u ${USER} -p ${ROOT_PASS} declare queue --vhost=/ name=some_outgoing_queue durable=true > /dev/null
rabbitmqadmin -u ${USER} -p ${ROOT_PASS} declare binding source="some_exchange" destination_type="queue" destination="some_outgoing_queue" routing_key="some_routing_key" > /dev/null

echo "push message to the queue. . ."
for((i=1;i<=20;i++));
do
rabbitmqadmin publish exchange="some_exchange" routing_key="some_routing_key" payload="Hello World" > /dev/null;
done

echo "get message from the queue. . ."
for((i=1;i<=10;i++));
do
rabbitmqadmin get queue=some_outgoing_queue ackmode=ack_requeue_false > /dev/null;
done
