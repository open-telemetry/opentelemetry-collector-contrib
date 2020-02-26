from redis import StrictRedis

from ...utils.deprecation import deprecated


DEFAULT_SERVICE = 'redis'


@deprecated(message='Use patching instead (see the docs).', version='1.0.0')
def get_traced_redis(ddtracer, service=DEFAULT_SERVICE, meta=None):
    return _get_traced_redis(ddtracer, StrictRedis, service, meta)


@deprecated(message='Use patching instead (see the docs).', version='1.0.0')
def get_traced_redis_from(ddtracer, baseclass, service=DEFAULT_SERVICE, meta=None):
    return _get_traced_redis(ddtracer, baseclass, service, meta)


def _get_traced_redis(ddtracer, baseclass, service, meta):
    return baseclass
