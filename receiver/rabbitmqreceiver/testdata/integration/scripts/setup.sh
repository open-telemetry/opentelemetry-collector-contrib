#!/usr/bin/env bash

set -e

USER='otelu'
ROOT_PASS='otelp'

echo "creating user: ${USER} . . ."
rabbitmqctl add_user ${USER} ${ROOT_PASS} > /dev/null

echo "Configuring ${USER} permissions. . ."
rabbitmqctl set_user_tags ${USER} administrator > /dev/null
rabbitmqctl set_permissions -p /  ${USER} ".*" ".*" ".*" > /dev/null

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
