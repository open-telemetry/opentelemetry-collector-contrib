__all__ = ['reverse']

try:
    from django.core.urlresolvers import reverse
except ImportError:
    from django.urls import reverse
