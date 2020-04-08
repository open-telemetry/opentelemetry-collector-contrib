import sqlalchemy

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .engine import _wrap_create_engine
from ...utils.wrappers import unwrap


def patch():
    if getattr(sqlalchemy.engine, '__datadog_patch', False):
        return
    setattr(sqlalchemy.engine, '__datadog_patch', True)

    # patch the engine creation function
    _w('sqlalchemy', 'create_engine', _wrap_create_engine)
    _w('sqlalchemy.engine', 'create_engine', _wrap_create_engine)


def unpatch():
    # unpatch sqlalchemy
    if getattr(sqlalchemy.engine, '__datadog_patch', False):
        setattr(sqlalchemy.engine, '__datadog_patch', False)
        unwrap(sqlalchemy, 'create_engine')
        unwrap(sqlalchemy.engine, 'create_engine')
