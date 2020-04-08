from unittest import TestCase

# Test base adapted from molten/tests/test_dependency_injection.py

from inspect import Parameter

import molten
from molten import DependencyInjector

from ddtrace import Pin
from ddtrace.contrib.molten import patch, unpatch

from ...test_tracer import get_dummy_tracer


class Settings(dict):
    pass


class SettingsComponent:
    is_singleton = True

    def can_handle_parameter(self, parameter: Parameter) -> bool:
        return parameter.annotation is Settings

    def resolve(self) -> Settings:
        return Settings()


class Metrics:
    __slots__ = ['settings']

    def __init__(self, settings: Settings) -> None:
        self.settings = settings


class MetricsComponent:
    is_singleton = True

    def can_handle_parameter(self, parameter: Parameter) -> bool:
        return parameter.annotation is Metrics

    def resolve(self, settings: Settings) -> Metrics:
        return Metrics(settings)


class DB:
    __slots__ = ['settings', 'metrics']

    def __init__(self, settings: Settings, metrics: Metrics) -> None:
        self.settings = settings
        self.metrics = metrics


class DBComponent:
    is_singleton = True

    def can_handle_parameter(self, parameter: Parameter) -> bool:
        return parameter.annotation is DB

    def resolve(self, settings: Settings, metrics: Metrics) -> DB:
        return DB(settings, metrics)


class Accounts:
    def __init__(self, db: DB) -> None:
        self.db = db

    def get_all(self):
        return []


class AccountsComponent:
    def can_handle_parameter(self, parameter: Parameter) -> bool:
        return parameter.annotation is Accounts

    def resolve(self, db: DB) -> Accounts:
        return Accounts(db)


class TestMoltenDI(TestCase):
    """"Ensures Molten dependency injection is properly instrumented."""

    TEST_SERVICE = 'molten-patch-di'

    def setUp(self):
        patch()
        self.tracer = get_dummy_tracer()
        Pin.override(molten, tracer=self.tracer, service=self.TEST_SERVICE)

    def tearDown(self):
        unpatch()
        self.tracer.writer.pop()

    def test_di_can_inject_dependencies(self):
        # Given that I have a DI instance
        di = DependencyInjector(components=[
            SettingsComponent(),
            MetricsComponent(),
            DBComponent(),
            AccountsComponent(),
        ])

        # And a function that uses DI
        def example(accounts: Accounts):
            assert accounts.get_all() == []
            return accounts

        # When I resolve that function
        # Then all the parameters should resolve as expected
        resolver = di.get_resolver()
        resolved_example = resolver.resolve(example)
        resolved_example()

        spans = self.tracer.writer.pop()

        # TODO[tahir]: We could in future trace the resolve method on components
        self.assertEqual(len(spans), 0)
