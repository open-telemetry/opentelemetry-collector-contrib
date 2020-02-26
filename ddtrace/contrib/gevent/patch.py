import gevent
import gevent.pool
import ddtrace

from .greenlet import TracedGreenlet, TracedIMap, TracedIMapUnordered, GEVENT_VERSION
from .provider import GeventContextProvider
from ...provider import DefaultContextProvider


__Greenlet = gevent.Greenlet
__IMap = gevent.pool.IMap
__IMapUnordered = gevent.pool.IMapUnordered


def patch():
    """
    Patch the gevent module so that all references to the
    internal ``Greenlet`` class points to the ``DatadogGreenlet``
    class.

    This action ensures that if a user extends the ``Greenlet``
    class, the ``TracedGreenlet`` is used as a parent class.
    """
    _replace(TracedGreenlet, TracedIMap, TracedIMapUnordered)
    ddtrace.tracer.configure(context_provider=GeventContextProvider())


def unpatch():
    """
    Restore the original ``Greenlet``. This function must be invoked
    before executing application code, otherwise the ``DatadogGreenlet``
    class may be used during initialization.
    """
    _replace(__Greenlet, __IMap, __IMapUnordered)
    ddtrace.tracer.configure(context_provider=DefaultContextProvider())


def _replace(g_class, imap_class, imap_unordered_class):
    """
    Utility function that replace the gevent Greenlet class with the given one.
    """
    # replace the original Greenlet classes with the new one
    gevent.greenlet.Greenlet = g_class

    if GEVENT_VERSION >= (1, 3):
        # For gevent >= 1.3.0, IMap and IMapUnordered were pulled out of
        # gevent.pool and into gevent._imap
        gevent._imap.IMap = imap_class
        gevent._imap.IMapUnordered = imap_unordered_class
        gevent.pool.IMap = gevent._imap.IMap
        gevent.pool.IMapUnordered = gevent._imap.IMapUnordered
        gevent.pool.Greenlet = gevent.greenlet.Greenlet
    else:
        # For gevent < 1.3, only patching of gevent.pool classes necessary
        gevent.pool.IMap = imap_class
        gevent.pool.IMapUnordered = imap_unordered_class

    gevent.pool.Group.greenlet_class = g_class

    # replace gevent shortcuts
    gevent.Greenlet = gevent.greenlet.Greenlet
    gevent.spawn = gevent.greenlet.Greenlet.spawn
    gevent.spawn_later = gevent.greenlet.Greenlet.spawn_later
