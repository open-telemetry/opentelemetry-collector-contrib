# project
from ddtrace.contrib.django.conf import DatadogSettings

# testing
from .utils import DjangoTraceTestCase


class DjangoInstrumentationTest(DjangoTraceTestCase):
    """
    Ensures that Django is correctly configured according to
    users settings
    """
    def test_tracer_flags(self):
        assert self.tracer.enabled
        assert self.tracer.writer.api.hostname == 'localhost'
        assert self.tracer.writer.api.port == 8126
        assert self.tracer.tags == {'env': 'test'}

    def test_environment_vars(self):
        # Django defaults can be overridden by env vars, ensuring that
        # environment strings are properly converted
        with self.override_env(dict(
            DATADOG_TRACE_AGENT_HOSTNAME='agent.consul.local',
            DATADOG_TRACE_AGENT_PORT='58126'
        )):
            settings = DatadogSettings()
            assert settings.AGENT_HOSTNAME == 'agent.consul.local'
            assert settings.AGENT_PORT == 58126

    def test_environment_var_wrong_port(self):
        # ensures that a wrong Agent Port doesn't crash the system
        # and defaults to 8126
        with self.override_env(dict(DATADOG_TRACE_AGENT_PORT='something')):
            settings = DatadogSettings()
            assert settings.AGENT_PORT == 8126
