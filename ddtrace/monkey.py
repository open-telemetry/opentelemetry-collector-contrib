"""Patch librairies to be automatically instrumented.

It can monkey patch supported standard libraries and third party modules.
A patched module will automatically report spans with its default configuration.

A library instrumentation can be configured (for instance, to report as another service)
using Pin. For that, check its documentation.
"""
import importlib
import sys
import threading

from ddtrace.vendor.wrapt.importer import when_imported

from .internal.logger import get_logger


log = get_logger(__name__)

# Default set of modules to automatically patch or not
PATCH_MODULES = {
    'asyncio': False,
    'boto': True,
    'botocore': True,
    'bottle': False,
    'cassandra': True,
    'celery': True,
    'consul': True,
    'elasticsearch': True,
    'algoliasearch': True,
    'futures': False,  # experimental propagation
    'grpc': True,
    'mongoengine': True,
    'mysql': True,
    'mysqldb': True,
    'pymysql': True,
    'psycopg': True,
    'pylibmc': True,
    'pymemcache': True,
    'pymongo': True,
    'redis': True,
    'rediscluster': True,
    'requests': True,
    'sqlalchemy': False,  # Prefer DB client instrumentation
    'sqlite3': True,
    'aiohttp': True,  # requires asyncio (Python 3.4+)
    'aiopg': True,
    'aiobotocore': False,
    'httplib': False,
    'vertica': True,
    'molten': True,
    'jinja2': True,
    'mako': True,
    'flask': True,
    'kombu': False,

    # Ignore some web framework integrations that might be configured explicitly in code
    'django': False,
    'falcon': False,
    'pylons': False,
    'pyramid': False,

    # Standard library modules off by default
    'logging': False,
}

_LOCK = threading.Lock()
_PATCHED_MODULES = set()

# Modules which are patched on first use
# DEV: These modules are patched when the user first imports them, rather than
#      explicitly importing and patching them on application startup `ddtrace.patch_all(module=True)`
# DEV: This ensures we do not patch a module until it is needed
# DEV: <contrib name> => <list of module names that trigger a patch>
_PATCH_ON_IMPORT = {
    'celery': ('celery', ),
    'flask': ('flask, '),
    'gevent': ('gevent', ),
    'requests': ('requests', ),
}


class PatchException(Exception):
    """Wraps regular `Exception` class when patching modules"""
    pass


def _on_import_factory(module, raise_errors=True):
    """Factory to create an import hook for the provided module name"""
    def on_import(hook):
        # Import and patch module
        path = 'ddtrace.contrib.%s' % module
        imported_module = importlib.import_module(path)
        imported_module.patch()

    return on_import


def patch_all(**patch_modules):
    """Automatically patches all available modules.

    :param dict patch_modules: Override whether particular modules are patched or not.

        >>> patch_all(redis=False, cassandra=False)
    """
    modules = PATCH_MODULES.copy()
    modules.update(patch_modules)

    patch(raise_errors=False, **modules)


def patch(raise_errors=True, **patch_modules):
    """Patch only a set of given modules.

    :param bool raise_errors: Raise error if one patch fail.
    :param dict patch_modules: List of modules to patch.

        >>> patch(psycopg=True, elasticsearch=True)
    """
    modules = [m for (m, should_patch) in patch_modules.items() if should_patch]
    for module in modules:
        if module in _PATCH_ON_IMPORT:
            # If the module has already been imported then patch immediately
            if module in sys.modules:
                patch_module(module, raise_errors=raise_errors)

            # Otherwise, add a hook to patch when it is imported for the first time
            else:
                # Use factory to create handler to close over `module` and `raise_errors` values from this loop
                when_imported(module)(_on_import_factory(module, raise_errors))

                # manually add module to patched modules
                _PATCHED_MODULES.add(module)
        else:
            patch_module(module, raise_errors=raise_errors)

    patched_modules = get_patched_modules()
    log.info(
        'patched %s/%s modules (%s)',
        len(patched_modules),
        len(modules),
        ','.join(patched_modules),
    )


def patch_module(module, raise_errors=True):
    """Patch a single module

    Returns if the module got properly patched.
    """
    try:
        return _patch_module(module)
    except Exception:
        if raise_errors:
            raise
        log.debug('failed to patch %s', module, exc_info=True)
        return False


def get_patched_modules():
    """Get the list of patched modules"""
    with _LOCK:
        return sorted(_PATCHED_MODULES)


def _patch_module(module):
    """_patch_module will attempt to monkey patch the module.

    Returns if the module got patched.
    Can also raise errors if it fails.
    """
    path = 'ddtrace.contrib.%s' % module
    with _LOCK:
        if module in _PATCHED_MODULES and module not in _PATCH_ON_IMPORT:
            log.debug('already patched: %s', path)
            return False

        try:
            imported_module = importlib.import_module(path)
            imported_module.patch()
        except ImportError:
            # if the import fails, the integration is not available
            raise PatchException('integration not available')
        except AttributeError:
            # if patch() is not available in the module, it means
            # that the library is not installed in the environment
            raise PatchException('module not installed')

        _PATCHED_MODULES.add(module)
        return True
