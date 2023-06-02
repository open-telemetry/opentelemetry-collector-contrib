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

USER="otel"
ROOT_PASS="otel"
CODE=1


setup_permissions() {
    # NOTE: -pPASSWORD is missing a space on purpose
    mysql -u root -p"${ROOT_PASS}" -e "GRANT PROCESS ON *.* TO ${USER}" > /dev/null
    mysql -u root -p"${ROOT_PASS}" -e "GRANT SELECT ON INFORMATION_SCHEMA.INNODB_METRICS TO ${USER}" > /dev/null
    mysql -u root -p"${ROOT_PASS}" -e "FLUSH PRIVILEGES" > /dev/null
}

setup_data() {
    mysql -u root -p"${ROOT_PASS}" -e "CREATE DATABASE a_schema" > /dev/null
    mysql -u root -p"${ROOT_PASS}" -e "CREATE TABLE a_schema.a_table (k int, v int)" > /dev/null
    mysql -u root -p"${ROOT_PASS}" -e "CREATE INDEX an_index ON a_schema.a_table ((k + v))" > /dev/null
}

echo "Configuring ${USER} permissions. . ."
end=$((SECONDS+60))
while [ $SECONDS -lt $end ]; do
    result="$?"
    if setup_permissions; then
        echo "Permissions configured!"
        while [ $SECONDS -lt $end ]; do
            result="$?"
            if setup_data; then
                echo "Data created!"
                exit 0
            fi
            echo "Trying again in 5 seconds. . ."
            sleep 5
        done
    fi
    echo "Trying again in 5 seconds. . ."
    sleep 5
done

echo "Failed to configure permissions"
