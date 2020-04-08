import abc
from ddtrace.vendor import six

from .internal.context_manager import DefaultContextManager


class BaseContextProvider(six.with_metaclass(abc.ABCMeta)):
    """
    A ``ContextProvider`` is an interface that provides the blueprint
    for a callable class, capable to retrieve the current active
    ``Context`` instance. Context providers must inherit this class
    and implement:
    * the ``active`` method, that returns the current active ``Context``
    * the ``activate`` method, that sets the current active ``Context``
    """
    @abc.abstractmethod
    def _has_active_context(self):
        pass

    @abc.abstractmethod
    def activate(self, context):
        pass

    @abc.abstractmethod
    def active(self):
        pass

    def __call__(self, *args, **kwargs):
        """Method available for backward-compatibility. It proxies the call to
        ``self.active()`` and must not do anything more.
        """
        return self.active()


class DefaultContextProvider(BaseContextProvider):
    """
    Default context provider that retrieves all contexts from the current
    thread-local storage. It is suitable for synchronous programming and
    Python WSGI frameworks.
    """
    def __init__(self, reset_context_manager=True):
        self._local = DefaultContextManager(reset=reset_context_manager)

    def _has_active_context(self):
        """
        Check whether we have a currently active context.

        :returns: Whether we have an active context
        :rtype: bool
        """
        return self._local._has_active_context()

    def activate(self, context):
        """Makes the given ``context`` active, so that the provider calls
        the thread-local storage implementation.
        """
        return self._local.set(context)

    def active(self):
        """Returns the current active ``Context`` for this tracer. Returned
        ``Context`` must be thread-safe or thread-local for this specific
        implementation.
        """
        return self._local.get()
