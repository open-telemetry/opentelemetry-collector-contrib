"""
This module is based off of wrapt.importer (wrapt==1.11.0)
https://github.com/GrahamDumpleton/wrapt/blob/4bcd190457c89e993ffcfec6dad9e9969c033e9e/src/wrapt/importer.py#L127-L136

The reasoning for this is that wrapt.importer does not provide a mechanism to
remove the import hooks and that wrapt removes the hooks after they are fired.

So this module differs from wrapt.importer in that:
    - removes unnecessary functionality (like allowing hooks to be import paths)
    - deregister_post_import_hook is introduced to remove hooks
    - the values of _post_import_hooks can only be lists (instead of allowing None)
    - notify_module_loaded is modified to not remove the hooks when they are
      fired.
"""
import sys
import threading

from ..compat import PY3
from ..internal.logger import get_logger
from ..utils import get_module_name
from ..vendor.wrapt.decorators import synchronized


log = get_logger(__name__)


_post_import_hooks = {}
_post_import_hooks_init = False
_post_import_hooks_lock = threading.RLock()


@synchronized(_post_import_hooks_lock)
def register_post_import_hook(name, hook):
    """
    Registers a module import hook, ``hook`` for a module with name ``name``.

    If the module is already imported the hook is called immediately and a
    debug message is logged since this should not be expected in our use-case.

    :param name: Name of the module (full dotted path)
    :type name: str
    :param hook: Callable to be invoked with the module when it is imported.
    :type hook: Callable
    :return:
    """
    # Automatically install the import hook finder if it has not already
    # been installed.
    global _post_import_hooks_init

    if not _post_import_hooks_init:
        _post_import_hooks_init = True
        sys.meta_path.insert(0, ImportHookFinder())

    hooks = _post_import_hooks.get(name, [])

    if hook in hooks:
        log.debug('hook "%s" already exists on module "%s"', hook, name)
        return

    module = sys.modules.get(name, None)

    # If the module has been imported already fire the hook and log a debug msg.
    if module:
        log.debug('module "%s" already imported, firing hook', name)
        hook(module)

    hooks.append(hook)
    _post_import_hooks[name] = hooks


@synchronized(_post_import_hooks_lock)
def notify_module_loaded(module):
    """
    Indicate that a module has been loaded. Any post import hooks which were
    registered for the target module will be invoked.

    Any raised exceptions will be caught and an error message indicating that
    the hook failed.

    :param module: The module being loaded
    :type module: ``types.ModuleType``
    """
    name = get_module_name(module)
    hooks = _post_import_hooks.get(name, [])

    for hook in hooks:
        try:
            hook(module)
        except Exception:
            log.warning('hook "%s" for module "%s" failed', hook, name, exc_info=True)


class _ImportHookLoader(object):
    """
    A custom module import finder. This intercepts attempts to import
    modules and watches out for attempts to import target modules of
    interest. When a module of interest is imported, then any post import
    hooks which are registered will be invoked.
    """

    def load_module(self, fullname):
        module = sys.modules[fullname]
        notify_module_loaded(module)
        return module


class _ImportHookChainedLoader(object):
    def __init__(self, loader):
        self.loader = loader

    def load_module(self, fullname):
        module = self.loader.load_module(fullname)
        notify_module_loaded(module)
        return module


class ImportHookFinder:
    def __init__(self):
        self.in_progress = {}

    @synchronized(_post_import_hooks_lock)
    def find_module(self, fullname, path=None):
        # If the module being imported is not one we have registered
        # post import hooks for, we can return immediately. We will
        # take no further part in the importing of this module.

        if fullname not in _post_import_hooks:
            return None

        # When we are interested in a specific module, we will call back
        # into the import system a second time to defer to the import
        # finder that is supposed to handle the importing of the module.
        # We set an in progress flag for the target module so that on
        # the second time through we don't trigger another call back
        # into the import system and cause a infinite loop.

        if fullname in self.in_progress:
            return None

        self.in_progress[fullname] = True

        # Now call back into the import system again.

        try:
            if PY3:
                # For Python 3 we need to use find_spec().loader
                # from the importlib.util module. It doesn't actually
                # import the target module and only finds the
                # loader. If a loader is found, we need to return
                # our own loader which will then in turn call the
                # real loader to import the module and invoke the
                # post import hooks.
                try:
                    import importlib.util

                    loader = importlib.util.find_spec(fullname).loader
                except (ImportError, AttributeError):
                    loader = importlib.find_loader(fullname, path)
                if loader:
                    return _ImportHookChainedLoader(loader)
            else:
                # For Python 2 we don't have much choice but to
                # call back in to __import__(). This will
                # actually cause the module to be imported. If no
                # module could be found then ImportError will be
                # raised. Otherwise we return a loader which
                # returns the already loaded module and invokes
                # the post import hooks.
                __import__(fullname)
                return _ImportHookLoader()

        finally:
            del self.in_progress[fullname]


@synchronized(_post_import_hooks_lock)
def deregister_post_import_hook(modulename, hook):
    """
    Deregisters post import hooks for a module given the module name and a hook
    that was previously installed.

    :param modulename: Name of the module the hook is installed on.
    :type: str
    :param hook: The hook to remove (the function itself)
    :type hook: Callable
    :return: whether a hook was removed or not
    """
    if modulename not in _post_import_hooks:
        return False

    hooks = _post_import_hooks[modulename]

    try:
        hooks.remove(hook)
        return True
    except ValueError:
        return False
