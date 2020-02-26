import os

from .deprecation import deprecation


def get_env(integration, variable, default=None):
    """Retrieves environment variables value for the given integration. It must be used
    for consistency between integrations. The implementation is backward compatible
    with legacy nomenclature:

    * `DATADOG_` is a legacy prefix with lower priority
    * `DD_` environment variables have the highest priority
    * the environment variable is built concatenating `integration` and `variable`
      arguments
    * return `default` otherwise

    """
    key = "{}_{}".format(integration, variable).upper()
    legacy_env = "DATADOG_{}".format(key)
    env = "DD_{}".format(key)

    value = os.getenv(env)
    legacy = os.getenv(legacy_env)
    if legacy:
        # Deprecation: `DATADOG_` variables are deprecated
        deprecation(
            name="DATADOG_", message="Use `DD_` prefix instead", version="1.0.0",
        )

    value = value or legacy
    return value if value else default


def deep_getattr(obj, attr_string, default=None):
    """
    Returns the attribute of `obj` at the dotted path given by `attr_string`
    If no such attribute is reachable, returns `default`

    >>> deep_getattr(cass, 'cluster')
    <cassandra.cluster.Cluster object at 0xa20c350

    >>> deep_getattr(cass, 'cluster.metadata.partitioner')
    u'org.apache.cassandra.dht.Murmur3Partitioner'

    >>> deep_getattr(cass, 'i.dont.exist', default='default')
    'default'
    """
    attrs = attr_string.split(".")
    for attr in attrs:
        try:
            obj = getattr(obj, attr)
        except AttributeError:
            return default

    return obj


def asbool(value):
    """Convert the given String to a boolean object.

    Accepted values are `True` and `1`.
    """
    if value is None:
        return False

    if isinstance(value, bool):
        return value

    return value.lower() in ("true", "1")


def flatten_dict(d, sep=".", prefix=""):
    """
    Returns a normalized dict of depth 1 with keys in order of embedding

    """
    # adapted from https://stackoverflow.com/a/19647596
    return (
        {prefix + sep + k if prefix else k: v for kk, vv in d.items() for k, v in flatten_dict(vv, sep, kk).items()}
        if isinstance(d, dict)
        else {prefix: d}
    )
