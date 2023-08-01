#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
# A script which will record new login, virtual server, pool, node, and pool member responses from a specified Big-IP environment

ENDPOINT='https://localhost'
USER='bigipuser'
PASSWORD='bigippassword'

JSON_HEADER='Content-Type: application/json'

LOGIN_RESPONSE_FILE='login_response.json'
VIRTUAL_RESPONSE_FILE='virtual_servers_response.json'
VIRTUAL_STATS_RESPONSE_FILE='virtual_servers_stats_response.json'
POOL_STATS_RESPONSE_FILE='pools_stats_response.json'
NODE_STATS_RESPONSE_FILE='nodes_stats_response.json'
MEMBERS_STATS_RESPONSE_FILE_SUFFIX='_pool_members_stats_response.json'

# Record the login response and write it to a file
LOGIN_RESPONSE=$(curl -sk POST $ENDPOINT/mgmt/shared/authn/login -H $JSON_HEADER -d '{"username":"'$USER'","password":"'$PASSWORD'"}')
echo $LOGIN_RESPONSE | jq . > $LOGIN_RESPONSE_FILE

# Retrieve token from the respons and create header with it
TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token.token')
TOKEN_HEADER='X-F5-Auth-Token:'$TOKEN''

# Record the virtual servers response and write it to a file
VIRTUAL_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/virtual -H $JSON_HEADER -H $TOKEN_HEADER)
echo $VIRTUAL_RESPONSE | jq . > $VIRTUAL_RESPONSE_FILE

# Record the virtual servers stats response and write it to a file
VIRTUAL_STATS_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/virtual/stats -H $JSON_HEADER -H $TOKEN_HEADER)
echo $VIRTUAL_STATS_RESPONSE | jq . > $VIRTUAL_STATS_RESPONSE_FILE

# Record the pools stats response and write it to a file
POOL_STATS_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/pool/stats -H $JSON_HEADER -H $TOKEN_HEADER)
echo $POOL_STATS_RESPONSE | jq . > $POOL_STATS_RESPONSE_FILE

# Record the nodes stats response and write it to a file
NODE_STATS_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/node/stats -H $JSON_HEADER -H $TOKEN_HEADER)
echo $NODE_STATS_RESPONSE | jq . > $NODE_STATS_RESPONSE_FILE

#Get list of pools
POOL_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/pool -H $JSON_HEADER -H $TOKEN_HEADER)
POOL_LIST=$(echo $POOL_RESPONSE | jq -r '[.items[].fullPath]' | jq -c '.[]')

for row in $(echo "$POOL_LIST"); do
    # Record the pool members stats response and write it to a unique file
    POOL_URI_NAME=$(echo ${row} | jq -r ${1} | tr \/ \~)
    MEMBER_STATS_RESPONSE=$(curl -sk GET $ENDPOINT/mgmt/tm/ltm/pool/$POOL_URI_NAME/members/stats -H $JSON_HEADER -H $TOKEN_HEADER)
    MEMBER_FILE_PREFIX=$(echo $POOL_URI_NAME | tr \~ _)
    MEMBER_STATS_RESPONSE_FILE=$(echo $MEMBER_FILE_PREFIX$MEMBERS_STATS_RESPONSE_FILE_SUFFIX)
    echo $MEMBER_STATS_RESPONSE | jq . > $MEMBER_STATS_RESPONSE_FILE
done
