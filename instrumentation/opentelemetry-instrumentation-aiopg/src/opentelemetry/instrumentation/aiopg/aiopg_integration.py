import typing

import wrapt
from aiopg.utils import _ContextManager, _PoolAcquireContextManager

from opentelemetry.instrumentation.dbapi import (
    DatabaseApiIntegration,
    TracedCursor,
)
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import Status, StatusCode


# pylint: disable=abstract-method
class AsyncProxyObject(wrapt.ObjectProxy):
    def __aiter__(self):
        return self.__wrapped__.__aiter__()

    async def __anext__(self):
        result = await self.__wrapped__.__anext__()
        return result

    async def __aenter__(self):
        return await self.__wrapped__.__aenter__()

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        return await self.__wrapped__.__aexit__(exc_type, exc_val, exc_tb)

    def __await__(self):
        return self.__wrapped__.__await__()


class AiopgIntegration(DatabaseApiIntegration):
    async def wrapped_connection(
        self,
        connect_method: typing.Callable[..., typing.Any],
        args: typing.Tuple[typing.Any, typing.Any],
        kwargs: typing.Dict[typing.Any, typing.Any],
    ):
        """Add object proxy to connection object."""
        connection = await connect_method(*args, **kwargs)
        # pylint: disable=protected-access
        self.get_connection_attributes(connection._conn)
        return get_traced_connection_proxy(connection, self)

    async def wrapped_pool(self, create_pool_method, args, kwargs):
        pool = await create_pool_method(*args, **kwargs)
        async with pool.acquire() as connection:
            # pylint: disable=protected-access
            self.get_connection_attributes(connection._conn)
        return get_traced_pool_proxy(pool, self)


def get_traced_connection_proxy(
    connection, db_api_integration, *args, **kwargs
):
    # pylint: disable=abstract-method
    class TracedConnectionProxy(AsyncProxyObject):
        # pylint: disable=unused-argument
        def __init__(self, connection, *args, **kwargs):
            super().__init__(connection)

        def cursor(self, *args, **kwargs):
            coro = self._cursor(*args, **kwargs)
            return _ContextManager(coro)

        async def _cursor(self, *args, **kwargs):
            # pylint: disable=protected-access
            cursor = await self.__wrapped__._cursor(*args, **kwargs)
            return get_traced_cursor_proxy(cursor, db_api_integration)

    return TracedConnectionProxy(connection, *args, **kwargs)


def get_traced_pool_proxy(pool, db_api_integration, *args, **kwargs):
    # pylint: disable=abstract-method
    class TracedPoolProxy(AsyncProxyObject):
        # pylint: disable=unused-argument
        def __init__(self, pool, *args, **kwargs):
            super().__init__(pool)

        def acquire(self):
            """Acquire free connection from the pool."""
            coro = self._acquire()
            return _PoolAcquireContextManager(coro, self)

        async def _acquire(self):
            # pylint: disable=protected-access
            connection = await self.__wrapped__._acquire()
            return get_traced_connection_proxy(
                connection, db_api_integration, *args, **kwargs
            )

    return TracedPoolProxy(pool, *args, **kwargs)


class AsyncTracedCursor(TracedCursor):
    async def traced_execution(
        self,
        query_method: typing.Callable[..., typing.Any],
        *args: typing.Tuple[typing.Any, typing.Any],
        **kwargs: typing.Dict[typing.Any, typing.Any]
    ):

        with self._db_api_integration.get_tracer().start_as_current_span(
            self._db_api_integration.name, kind=SpanKind.CLIENT
        ) as span:
            self._populate_span(span, *args)
            try:
                result = await query_method(*args, **kwargs)
                return result
            except Exception as ex:  # pylint: disable=broad-except
                if span.is_recording():
                    span.set_status(Status(StatusCode.ERROR, str(ex)))
                raise ex


def get_traced_cursor_proxy(cursor, db_api_integration, *args, **kwargs):
    _traced_cursor = AsyncTracedCursor(db_api_integration)

    # pylint: disable=abstract-method
    class AsyncTracedCursorProxy(AsyncProxyObject):

        # pylint: disable=unused-argument
        def __init__(self, cursor, *args, **kwargs):
            super().__init__(cursor)

        async def execute(self, *args, **kwargs):
            result = await _traced_cursor.traced_execution(
                self.__wrapped__.execute, *args, **kwargs
            )
            return result

        async def executemany(self, *args, **kwargs):
            result = await _traced_cursor.traced_execution(
                self.__wrapped__.executemany, *args, **kwargs
            )
            return result

        async def callproc(self, *args, **kwargs):
            result = await _traced_cursor.traced_execution(
                self.__wrapped__.callproc, *args, **kwargs
            )
            return result

    return AsyncTracedCursorProxy(cursor, *args, **kwargs)
