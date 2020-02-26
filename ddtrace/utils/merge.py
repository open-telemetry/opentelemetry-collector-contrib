# Borrowed from: https://stackoverflow.com/questions/20656135/python-deep-merge-dictionary-data#20666342
def deepmerge(source, destination):
    """
    Merge the first provided ``dict`` into the second.

    :param dict source: The ``dict`` to merge into ``destination``
    :param dict destination: The ``dict`` that should get updated
    :rtype: dict
    :returns: ``destination`` modified
    """
    for key, value in source.items():
        if isinstance(value, dict):
            # get node or create one
            node = destination.setdefault(key, {})
            deepmerge(value, node)
        else:
            destination[key] = value

    return destination
