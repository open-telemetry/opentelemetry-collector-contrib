# Enabling Query Metrics

**Note that the Query Metrics only work on Postgres versions > 10. For more information please refer to the [pg_stat_statements](https://www.postgresql.org/docs/current/pgstatstatements.html) documentation.**

The query metrics (all metrics prefixed with `postgresql.query.*` rely on the `pg_stat_statements` extension to be enabled. To enable them:

For security reasons only super users and members of the `pg_read_all_stats` role are allowed to see the SQL text.

1. On the database issue this create extension command

    ```sql
    CREATE EXTENSION pg_stat_statements;
    ```

2. Modify the `shared_preload_libraries` field in the `postgresql.conf` to include the extension.

    ```conf
    shared_preload_libraries = 'pg_stat_statements'
    ```

3. Restart the instance to enable the extension. Now the query metrics can be collected.

