#!/usr/bin/env bash

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

echo "Configuring ${USER} permissions. . ."
end=$((SECONDS+60))
while [ $SECONDS -lt $end ]; do
    result="$?"
    if setup_permissions; then
        echo "Permissions configured!"
        exit 0
    fi
    echo "Trying again in 5 seconds. . ."
    sleep 5
done

echo "Failed to configure permissions"