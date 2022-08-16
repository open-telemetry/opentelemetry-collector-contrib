#!/usr/bin/env bash

{
echo "shared_preload_libraries = 'pg_stat_statements'" 
echo "pg_stat_statements.max = 10000" 
echo "pg_stat_statements.track = all"
} >> "$PGDATA"/postgresql.conf