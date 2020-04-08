import dogpile

from ddtrace.pin import Pin, _DD_PIN_NAME, _DD_PIN_PROXY_NAME
from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .lock import _wrap_lock_ctor
from .region import _wrap_get_create, _wrap_get_create_multi

_get_or_create = dogpile.cache.region.CacheRegion.get_or_create
_get_or_create_multi = dogpile.cache.region.CacheRegion.get_or_create_multi
_lock_ctor = dogpile.lock.Lock.__init__


def patch():
    if getattr(dogpile.cache, '_datadog_patch', False):
        return
    setattr(dogpile.cache, '_datadog_patch', True)

    _w('dogpile.cache.region', 'CacheRegion.get_or_create', _wrap_get_create)
    _w('dogpile.cache.region', 'CacheRegion.get_or_create_multi', _wrap_get_create_multi)
    _w('dogpile.lock', 'Lock.__init__', _wrap_lock_ctor)

    Pin(app='dogpile.cache', service='dogpile.cache').onto(dogpile.cache)


def unpatch():
    if not getattr(dogpile.cache, '_datadog_patch', False):
        return
    setattr(dogpile.cache, '_datadog_patch', False)
    # This looks silly but the unwrap util doesn't support class instance methods, even
    # though wrapt does. This was causing the patches to stack on top of each other
    # during testing.
    dogpile.cache.region.CacheRegion.get_or_create = _get_or_create
    dogpile.cache.region.CacheRegion.get_or_create_multi = _get_or_create_multi
    dogpile.lock.Lock.__init__ = _lock_ctor
    setattr(dogpile.cache, _DD_PIN_NAME, None)
    setattr(dogpile.cache, _DD_PIN_PROXY_NAME, None)
