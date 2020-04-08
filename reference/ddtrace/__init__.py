import sys

import pkg_resources

from .monkey import patch, patch_all
from .pin import Pin
from .span import Span
from .tracer import Tracer
from .settings import config


try:
    __version__ = pkg_resources.get_distribution(__name__).version
except pkg_resources.DistributionNotFound:
    # package is not installed
    __version__ = None


# a global tracer instance with integration settings
tracer = Tracer()

__all__ = [
    'patch',
    'patch_all',
    'Pin',
    'Span',
    'tracer',
    'Tracer',
    'config',
]


_ORIGINAL_EXCEPTHOOK = sys.excepthook


def _excepthook(tp, value, traceback):
    tracer.global_excepthook(tp, value, traceback)
    if _ORIGINAL_EXCEPTHOOK:
        return _ORIGINAL_EXCEPTHOOK(tp, value, traceback)


def install_excepthook():
    """Install a hook that intercepts unhandled exception and send metrics about them."""
    global _ORIGINAL_EXCEPTHOOK
    _ORIGINAL_EXCEPTHOOK = sys.excepthook
    sys.excepthook = _excepthook


def uninstall_excepthook():
    """Uninstall the global tracer except hook."""
    sys.excepthook = _ORIGINAL_EXCEPTHOOK
