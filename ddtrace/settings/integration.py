from copy import deepcopy

from ..utils.attrdict import AttrDict
from ..utils.formats import asbool, get_env
from .http import HttpConfig
from .hooks import Hooks


class IntegrationConfig(AttrDict):
    """
    Integration specific configuration object.

    This is what you will get when you do::

        from ddtrace import config

        # This is an `IntegrationConfig`
        config.flask

        # `IntegrationConfig` supports both attribute and item accessors
        config.flask['service_name'] = 'my-service-name'
        config.flask.service_name = 'my-service-name'
    """
    def __init__(self, global_config, name, *args, **kwargs):
        """
        :param global_config:
        :type global_config: Config
        :param args:
        :param kwargs:
        """
        super(IntegrationConfig, self).__init__(*args, **kwargs)

        # Set internal properties for this `IntegrationConfig`
        # DEV: By-pass the `__setattr__` overrides from `AttrDict` to set real properties
        object.__setattr__(self, 'global_config', global_config)
        object.__setattr__(self, 'integration_name', name)
        object.__setattr__(self, 'hooks', Hooks())
        object.__setattr__(self, 'http', HttpConfig())

        # Set default analytics configuration, default is disabled
        # DEV: Default to `None` which means do not set this key
        # Inject environment variables for integration
        analytics_enabled_env = get_env(name, 'analytics_enabled')
        if analytics_enabled_env is not None:
            analytics_enabled_env = asbool(analytics_enabled_env)
        self.setdefault('analytics_enabled', analytics_enabled_env)
        self.setdefault('analytics_sample_rate', float(get_env(name, 'analytics_sample_rate', 1.0)))

    def __deepcopy__(self, memodict=None):
        new = IntegrationConfig(self.global_config, deepcopy(dict(self)))
        new.hooks = deepcopy(self.hooks)
        new.http = deepcopy(self.http)
        return new

    @property
    def trace_query_string(self):
        if self.http.trace_query_string is not None:
            return self.http.trace_query_string
        return self.global_config._http.trace_query_string

    def header_is_traced(self, header_name):
        """
        Returns whether or not the current header should be traced.
        :param header_name: the header name
        :type header_name: str
        :rtype: bool
        """
        return (
            self.http.header_is_traced(header_name)
            if self.http.is_header_tracing_configured
            else self.global_config.header_is_traced(header_name)
        )

    def _is_analytics_enabled(self, use_global_config):
        # DEV: analytics flag can be None which should not be taken as
        # enabled when global flag is disabled
        if use_global_config and self.global_config.analytics_enabled:
            return self.analytics_enabled is not False
        else:
            return self.analytics_enabled is True

    def get_analytics_sample_rate(self, use_global_config=False):
        """
        Returns analytics sample rate but only when integration-specific
        analytics configuration is enabled with optional override with global
        configuration
        """
        if self._is_analytics_enabled(use_global_config):
            analytics_sample_rate = getattr(self, 'analytics_sample_rate', None)
            # return True if attribute is None or attribute not found
            if analytics_sample_rate is None:
                return True
            # otherwise return rate
            return analytics_sample_rate

        # Use `None` as a way to say that it was not defined,
        #   `False` would mean `0` which is a different thing
        return None

    def __repr__(self):
        cls = self.__class__
        keys = ', '.join(self.keys())
        return '{}.{}({})'.format(cls.__module__, cls.__name__, keys)
