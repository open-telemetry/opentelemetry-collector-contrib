# [Backward compatibility]: keep importing modules functions
from ..utils.deprecation import deprecation
from ..utils.importlib import require_modules, func_name, module_name


deprecation(
    name='ddtrace.contrib.util',
    message='Use `ddtrace.utils.importlib` module instead',
    version='1.0.0',
)

__all__ = [
    'require_modules',
    'func_name',
    'module_name',
]
