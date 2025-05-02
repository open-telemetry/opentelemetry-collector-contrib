#!/bin/bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
#

DBSTATUS=1
ERRCODE=1
i=0


while [[ $DBSTATUS -ne 0 ]] && [[ $i -lt 60 ]]; do
	i=$((i + 1))
	DBSTATUS=$(/opt/mssql-tools18/bin/sqlcmd -h -1 -t 1 -U sa -P "${SA_PASSWORD}" -C -Q "SET NOCOUNT ON; Select SUM(state) from sys.databases")
	DBSTATUS=${DBSTATUS:-1}
	sleep 1
done

if [[ $DBSTATUS -ne 0 ]]; then
	echo "SQL Server took more than 60 seconds to start up or one or more databases are not in an ONLINE state"
	exit 1
fi

# Run the setup script to create the DB and the schema in the DB
/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "${SA_PASSWORD}" -C -d master -i /usr/src/app/init.sql
