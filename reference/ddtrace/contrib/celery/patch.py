import celery

from ddtrace import config

from .app import patch_app, unpatch_app
from .constants import PRODUCER_SERVICE, WORKER_SERVICE
from ...utils.formats import get_env


# Celery default settings
config._add('celery', {
    'producer_service_name': get_env('celery', 'producer_service_name', PRODUCER_SERVICE),
    'worker_service_name': get_env('celery', 'worker_service_name', WORKER_SERVICE),
})


def patch():
    """Instrument Celery base application and the `TaskRegistry` so
    that any new registered task is automatically instrumented. In the
    case of Django-Celery integration, also the `@shared_task` decorator
    must be instrumented because Django doesn't use the Celery registry.
    """
    patch_app(celery.Celery)


def unpatch():
    """Disconnect all signals and remove Tracing capabilities"""
    unpatch_app(celery.Celery)
