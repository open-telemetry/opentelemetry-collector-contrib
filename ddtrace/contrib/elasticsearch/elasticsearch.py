from importlib import import_module

module_names = ('elasticsearch', 'elasticsearch1', 'elasticsearch2', 'elasticsearch5', 'elasticsearch6')
for module_name in module_names:
    try:
        elasticsearch = import_module(module_name)
        break
    except ImportError:
        pass
else:
    raise ImportError('could not import any of {0!r}'.format(module_names))


__all__ = ['elasticsearch']
