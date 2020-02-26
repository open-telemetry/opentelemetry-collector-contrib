# [Backward compatibility]: keep importing modules functions
from .utils.deprecation import deprecated, deprecation
from .utils.formats import asbool, deep_getattr, get_env
from .utils.wrappers import safe_patch, unwrap


deprecation(
    name='ddtrace.util',
    message='Use `ddtrace.utils` package instead',
    version='1.0.0',
)

__all__ = [
    'deprecated',
    'asbool',
    'deep_getattr',
    'get_env',
    'safe_patch',
    'unwrap',
]
