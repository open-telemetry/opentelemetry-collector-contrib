import collections
from copy import deepcopy

from ..internal.logger import get_logger
from ..span import Span

log = get_logger(__name__)


class Hooks(object):
    """
    Hooks configuration object is used for registering and calling hook functions

    Example::

        @config.falcon.hooks.on('request')
        def on_request(span, request, response):
            pass
    """
    __slots__ = ['_hooks']

    def __init__(self):
        self._hooks = collections.defaultdict(set)

    def __deepcopy__(self, memodict=None):
        hooks = Hooks()
        hooks._hooks = deepcopy(self._hooks)
        return hooks

    def register(self, hook, func=None):
        """
        Function used to register a hook for the provided name.

        Example::

            def on_request(span, request, response):
                pass

            config.falcon.hooks.register('request', on_request)


        If no function is provided then a decorator is returned::

            @config.falcon.hooks.register('request')
            def on_request(span, request, response):
                pass

        :param hook: The name of the hook to register the function for
        :type hook: str
        :param func: The function to register, or ``None`` if a decorator should be returned
        :type func: function, None
        :returns: Either a function decorator if ``func is None``, otherwise ``None``
        :rtype: function, None
        """
        # If they didn't provide a function, then return a decorator
        if not func:
            def wrapper(func):
                self.register(hook, func)
                return func
            return wrapper
        self._hooks[hook].add(func)

    # Provide shorthand `on` method for `register`
    # >>> @config.falcon.hooks.on('request')
    #     def on_request(span, request, response):
    #        pass
    on = register

    def deregister(self, func):
        """
        Function to deregister a function from all hooks it was registered under

        Example::

            @config.falcon.hooks.on('request')
            def on_request(span, request, response):
                pass

            config.falcon.hooks.deregister(on_request)


        :param func: Function hook to register
        :type func: function
        """
        for funcs in self._hooks.values():
            if func in funcs:
                funcs.remove(func)

    def _emit(self, hook, span, *args, **kwargs):
        """
        Function used to call registered hook functions.

        :param hook: The hook to call functions for
        :type hook: str
        :param span: The span to call the hook with
        :type span: :class:`ddtrace.span.Span`
        :param args: Positional arguments to pass to the hook functions
        :type args: list
        :param kwargs: Keyword arguments to pass to the hook functions
        :type kwargs: dict
        """
        # Return early if no hooks are registered
        if hook not in self._hooks:
            return

        # Return early if we don't have a Span
        if not isinstance(span, Span):
            return

        # Call registered hooks
        for func in self._hooks[hook]:
            try:
                func(span, *args, **kwargs)
            except Exception:
                # DEV: Use log.debug instead of log.error until we have a throttled logger
                log.debug('Failed to run hook %s function %s', hook, func, exc_info=True)

    def __repr__(self):
        """Return string representation of this class instance"""
        cls = self.__class__
        hooks = ','.join(self._hooks.keys())
        return '{}.{}({})'.format(cls.__module__, cls.__name__, hooks)
