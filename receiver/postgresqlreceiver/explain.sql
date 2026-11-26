-- Copyright The OpenTelemetry Authors
-- SPDX-License-Identifier: Apache-2.0

-- Adapted from newrelic-forks/opentelemetry-collector-contrib PR #243.
-- Run as a dedicated non-superuser role with privileges on the monitored tables.
-- Replace public below with the trusted schemas used by the captured queries.
BEGIN;

CREATE OR REPLACE FUNCTION otel.explain_statement(l_query text)
RETURNS json
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public, pg_temp
SET plan_cache_mode = force_generic_plan
AS $$
DECLARE
    v_plan json;
    v_param_count int;
    v_nulls text := '';
    v_prepared boolean := false;
    i int;
BEGIN
    -- A caller that can create temporary functions can make planning run them as the owner.
    IF pg_catalog.has_database_privilege(session_user, pg_catalog.current_database(), 'TEMPORARY') THEN
        RAISE EXCEPTION 'EXPLAIN helper refuses callers with the TEMPORARY privilege on this database'
            USING ERRCODE = '42501';
    END IF;

    -- Temporary functions survive changes to the caller's role or privileges.
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_proc
        WHERE pronamespace = pg_catalog.pg_my_temp_schema()
    ) THEN
        RAISE EXCEPTION 'EXPLAIN helper refuses sessions with temporary functions'
            USING ERRCODE = '42501';
    END IF;

    l_query := pg_catalog.regexp_replace(l_query, ';[[:space:]]*$', '');
    -- Reject separators even inside literals or comments rather than parse SQL here.
    -- PREPARE through dynamic EXECUTE otherwise accepts additional commands.
    IF pg_catalog.strpos(l_query, ';') > 0 THEN
        RAISE EXCEPTION 'EXPLAIN helper accepts one statement without semicolons'
            USING ERRCODE = '22023';
    END IF;

    SET TRANSACTION READ ONLY;
    EXECUTE 'PREPARE otel_explain_stmt AS ' || l_query;
    v_prepared := true;

    SELECT COALESCE(pg_catalog.array_length(parameter_types, 1), 0)
    INTO v_param_count
    FROM pg_catalog.pg_prepared_statements WHERE name = 'otel_explain_stmt';

    IF v_param_count > 0 THEN
        FOR i IN 1..v_param_count LOOP
            v_nulls := v_nulls || CASE WHEN i > 1 THEN ', ' ELSE '' END || 'null';
        END LOOP;
        v_nulls := '(' || v_nulls || ')';
    END IF;

    EXECUTE 'EXPLAIN (FORMAT JSON) EXECUTE otel_explain_stmt' || v_nulls INTO v_plan;
    DEALLOCATE otel_explain_stmt;
    RETURN v_plan;
EXCEPTION WHEN query_canceled OR OTHERS THEN
    IF v_prepared THEN
        DEALLOCATE otel_explain_stmt;
    END IF;
    RAISE;
END;
$$;

REVOKE ALL ON FUNCTION otel.explain_statement(text) FROM PUBLIC;

COMMIT;
