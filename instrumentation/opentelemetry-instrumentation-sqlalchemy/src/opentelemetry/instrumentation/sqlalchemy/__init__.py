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

"""
Instrument `sqlalchemy`_ to report SQL queries.

There are two options for instrumenting code. The first option is to use
the ``opentelemetry-instrument`` executable which will automatically
instrument your SQLAlchemy engine. The second is to programmatically enable
instrumentation via the following code:

.. _sqlalchemy: https://pypi.org/project/sqlalchemy/

Usage
-----
.. code:: python

    from sqlalchemy import create_engine

    from opentelemetry import trace
    from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor
    from opentelemetry.sdk.trace import TracerProvider
    import sqlalchemy

    trace.set_tracer_provider(TracerProvider())
    engine = create_engine("sqlite:///:memory:")
    SQLAlchemyInstrumentor().instrument(
        engine=engine,
        service="service-A",
    )

API
---
"""
import sqlalchemy
import wrapt
from wrapt import wrap_function_wrapper as _w

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.sqlalchemy.engine import (
    EngineTracer,
    _get_tracer,
    _wrap_create_engine,
)
from opentelemetry.instrumentation.utils import unwrap


class SQLAlchemyInstrumentor(BaseInstrumentor):
    """An instrumentor for SQLAlchemy
    See `BaseInstrumentor`
    """

    def _instrument(self, **kwargs):
        """Instruments SQLAlchemy engine creation methods and the engine
        if passed as an argument.

        Args:
            **kwargs: Optional arguments
                ``engine``: a SQLAlchemy engine instance
                ``tracer_provider``: a TracerProvider, defaults to global
                ``service``: the name of the service to trace.

        Returns:
            An instrumented engine if passed in as an argument, None otherwise.
        """
        _w("sqlalchemy", "create_engine", _wrap_create_engine)
        _w("sqlalchemy.engine", "create_engine", _wrap_create_engine)
        if kwargs.get("engine") is not None:
            return EngineTracer(
                _get_tracer(
                    kwargs.get("engine"), kwargs.get("tracer_provider")
                ),
                kwargs.get("service"),
                kwargs.get("engine"),
            )
        return None

    def _uninstrument(self, **kwargs):
        unwrap(sqlalchemy, "create_engine")
        unwrap(sqlalchemy.engine, "create_engine")
