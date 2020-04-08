"""
The Celery integration will trace all tasks that are executed in the
background. Functions and class based tasks are traced only if the Celery API
is used, so calling the function directly or via the ``run()`` method will not
generate traces. However, calling ``apply()``, ``apply_async()`` and ``delay()``
will produce tracing data. To trace your Celery application, call the patch method::

    import celery
    from ddtrace import patch

    patch(celery=True)
    app = celery.Celery()

    @app.task
    def my_task():
        pass

    class MyTask(app.Task):
        def run(self):
            pass


To change Celery service name, you can use the ``Config`` API as follows::

    from ddtrace import config

    # change service names for producers and workers
    config.celery['producer_service_name'] = 'task-queue'
    config.celery['worker_service_name'] = 'worker-notify'

By default, reported service names are:
    * ``celery-producer`` when tasks are enqueued for processing
    * ``celery-worker`` when tasks are processed by a Celery process

"""
from ...utils.importlib import require_modules


required_modules = ['celery']

with require_modules(required_modules) as missing_modules:
    if not missing_modules:
        from .app import patch_app, unpatch_app
        from .patch import patch, unpatch
        from .task import patch_task, unpatch_task

        __all__ = [
            'patch',
            'patch_app',
            'patch_task',
            'unpatch',
            'unpatch_app',
            'unpatch_task',
        ]
