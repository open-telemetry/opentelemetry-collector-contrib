#!/bin/bash

# Run the SQL script to initialize the database
/opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P YourStrong!Passw0rd -d master -i /usr/src/app/init.sql