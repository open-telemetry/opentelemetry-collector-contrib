# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
from functools import wraps

import pytest

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.celery import CeleryInstrumentor
from opentelemetry.sdk.trace import TracerProvider, export
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT ", "6379"))
REDIS_URL = "redis://{host}:{port}".format(host=REDIS_HOST, port=REDIS_PORT)
BROKER_URL = "{redis}/{db}".format(redis=REDIS_URL, db=0)
BACKEND_URL = "{redis}/{db}".format(redis=REDIS_URL, db=1)


@pytest.fixture(scope="session")
def celery_config():
    return {"broker_url": BROKER_URL, "result_backend": BACKEND_URL}


@pytest.fixture
def celery_worker_parameters():
    return {
        # See https://github.com/celery/celery/issues/3642#issuecomment-457773294
        "perform_ping_check": False,
    }


@pytest.fixture(autouse=True)
def patch_celery_app(celery_app, celery_worker):
    """Patch task decorator on app fixture to reload worker"""
    # See https://github.com/celery/celery/issues/3642
    def wrap_task(fn):
        @wraps(fn)
        def wrapper(*args, **kwargs):
            result = fn(*args, **kwargs)
            celery_worker.reload()
            return result

        return wrapper

    celery_app.task = wrap_task(celery_app.task)


@pytest.fixture(autouse=True)
def instrument(tracer_provider, memory_exporter):
    CeleryInstrumentor().instrument(tracer_provider=tracer_provider)
    memory_exporter.clear()

    yield

    CeleryInstrumentor().uninstrument()


@pytest.fixture(scope="session")
def tracer_provider(memory_exporter):
    original_tracer_provider = trace_api.get_tracer_provider()

    tracer_provider = TracerProvider()

    span_processor = export.SimpleExportSpanProcessor(memory_exporter)
    tracer_provider.add_span_processor(span_processor)

    trace_api.set_tracer_provider(tracer_provider)

    yield tracer_provider

    trace_api.set_tracer_provider(original_tracer_provider)


@pytest.fixture(scope="session")
def memory_exporter():
    memory_exporter = InMemorySpanExporter()
    return memory_exporter
