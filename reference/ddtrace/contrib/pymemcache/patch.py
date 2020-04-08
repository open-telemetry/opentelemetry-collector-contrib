import pymemcache

from ddtrace.ext import memcached as memcachedx
from ddtrace.pin import Pin, _DD_PIN_NAME, _DD_PIN_PROXY_NAME
from .client import WrappedClient

_Client = pymemcache.client.base.Client


def patch():
    if getattr(pymemcache.client, '_datadog_patch', False):
        return

    setattr(pymemcache.client, '_datadog_patch', True)
    setattr(pymemcache.client.base, 'Client', WrappedClient)

    # Create a global pin with default configuration for our pymemcache clients
    Pin(
        app=memcachedx.SERVICE, service=memcachedx.SERVICE
    ).onto(pymemcache)


def unpatch():
    """Remove pymemcache tracing"""
    if not getattr(pymemcache.client, '_datadog_patch', False):
        return
    setattr(pymemcache.client, '_datadog_patch', False)
    setattr(pymemcache.client.base, 'Client', _Client)

    # Remove any pins that may exist on the pymemcache reference
    setattr(pymemcache, _DD_PIN_NAME, None)
    setattr(pymemcache, _DD_PIN_PROXY_NAME, None)
