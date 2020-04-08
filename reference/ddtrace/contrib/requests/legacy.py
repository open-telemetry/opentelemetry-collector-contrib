# [Deprecation]: this module contains deprecated functions
# that will be removed in newer versions of the Tracer.
from ddtrace import config

from ...utils.deprecation import deprecation


def _distributed_tracing(self):
    """Deprecated: this method has been deprecated in favor of
    the configuration system. It will be removed in newer versions
    of the Tracer.
    """
    deprecation(
        name='client.distributed_tracing',
        message='Use the configuration object instead `config.get_from(client)[\'distributed_tracing\'`',
        version='1.0.0',
    )
    return config.get_from(self)['distributed_tracing']


def _distributed_tracing_setter(self, value):
    """Deprecated: this method has been deprecated in favor of
    the configuration system. It will be removed in newer versions
    of the Tracer.
    """
    deprecation(
        name='client.distributed_tracing',
        message='Use the configuration object instead `config.get_from(client)[\'distributed_tracing\'] = value`',
        version='1.0.0',
    )
    config.get_from(self)['distributed_tracing'] = value
