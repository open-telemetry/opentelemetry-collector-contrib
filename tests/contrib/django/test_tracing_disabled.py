# 3rd party
from django.apps import apps
from django.test import TestCase

# project
from ddtrace.tracer import Tracer
from ddtrace.contrib.django.conf import settings

# testing
from ...test_tracer import DummyWriter


class DjangoTracingDisabledTest(TestCase):
    def setUp(self):
        # backup previous conf
        self.backupEnabled = settings.ENABLED
        self.backupTracer = settings.TRACER

        # Use a new tracer to be sure that a new service
        # would be sent to the the writer
        self.tracer = Tracer()
        self.tracer.writer = DummyWriter()

        # Restart app with tracing disabled
        settings.ENABLED = False
        self.app = apps.get_app_config('datadog_django')
        self.app.ready()

    def tearDown(self):
        # Reset the original settings
        settings.ENABLED = self.backupEnabled
        settings.TRACER = self.backupTracer
        self.app.ready()

    def test_no_service_info_is_written(self):
        services = self.tracer.writer.pop_services()
        assert len(services) == 0

    def test_no_trace_is_written(self):
        settings.TRACER.trace('client.testing').finish()
        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 0
