import dogpile
import pytest

from ddtrace import Pin
from ddtrace.contrib.dogpile_cache.patch import patch, unpatch

from tests.test_tracer import get_dummy_tracer


@pytest.fixture
def tracer():
    return get_dummy_tracer()


@pytest.fixture
def region(tracer):
    patch()
    # Setup a simple dogpile cache region for testing.
    # The backend is trivial so we can use memory to simplify test setup.
    test_region = dogpile.cache.make_region(name="TestRegion")
    test_region.configure("dogpile.cache.memory")
    Pin.override(dogpile.cache, tracer=tracer)
    return test_region


@pytest.fixture(autouse=True)
def cleanup():
    yield
    unpatch()


@pytest.fixture
def single_cache(region):
    @region.cache_on_arguments()
    def fn(x):
        return x * 2

    return fn


@pytest.fixture
def multi_cache(region):
    @region.cache_multi_on_arguments()
    def fn(*x):
        return [i * 2 for i in x]

    return fn


def test_doesnt_trace_with_no_pin(tracer, single_cache, multi_cache):
    # No pin is set
    unpatch()

    assert single_cache(1) == 2
    assert tracer.writer.pop_traces() == []

    assert multi_cache(2, 3) == [4, 6]
    assert tracer.writer.pop_traces() == []


def test_doesnt_trace_with_disabled_pin(tracer, single_cache, multi_cache):
    tracer.enabled = False

    assert single_cache(1) == 2
    assert tracer.writer.pop_traces() == []

    assert multi_cache(2, 3) == [4, 6]
    assert tracer.writer.pop_traces() == []


def test_traces_get_or_create(tracer, single_cache):
    assert single_cache(1) == 2
    traces = tracer.writer.pop_traces()
    assert len(traces) == 1
    spans = traces[0]
    assert len(spans) == 1
    span = spans[0]
    assert span.name == "dogpile.cache"
    assert span.resource == "get_or_create"
    assert span.meta["key"] == "tests.contrib.dogpile_cache.test_tracing:fn|1"
    assert span.meta["hit"] == "False"
    assert span.meta["expired"] == "True"
    assert span.meta["backend"] == "MemoryBackend"
    assert span.meta["region"] == "TestRegion"

    # Now the results should be cached.
    assert single_cache(1) == 2
    traces = tracer.writer.pop_traces()
    assert len(traces) == 1
    spans = traces[0]
    assert len(spans) == 1
    span = spans[0]
    assert span.name == "dogpile.cache"
    assert span.resource == "get_or_create"
    assert span.meta["key"] == "tests.contrib.dogpile_cache.test_tracing:fn|1"
    assert span.meta["hit"] == "True"
    assert span.meta["expired"] == "False"
    assert span.meta["backend"] == "MemoryBackend"
    assert span.meta["region"] == "TestRegion"


def test_traces_get_or_create_multi(tracer, multi_cache):
    assert multi_cache(2, 3) == [4, 6]
    traces = tracer.writer.pop_traces()
    assert len(traces) == 1
    spans = traces[0]
    assert len(spans) == 1
    span = spans[0]
    assert span.meta["keys"] == (
        "['tests.contrib.dogpile_cache.test_tracing:fn|2', " + "'tests.contrib.dogpile_cache.test_tracing:fn|3']"
    )
    assert span.meta["hit"] == "False"
    assert span.meta["expired"] == "True"
    assert span.meta["backend"] == "MemoryBackend"
    assert span.meta["region"] == "TestRegion"

    # Partial hit
    assert multi_cache(2, 4) == [4, 8]
    traces = tracer.writer.pop_traces()
    assert len(traces) == 1
    spans = traces[0]
    assert len(spans) == 1
    span = spans[0]
    assert span.meta["keys"] == (
        "['tests.contrib.dogpile_cache.test_tracing:fn|2', " + "'tests.contrib.dogpile_cache.test_tracing:fn|4']"
    )
    assert span.meta["hit"] == "False"
    assert span.meta["expired"] == "True"
    assert span.meta["backend"] == "MemoryBackend"
    assert span.meta["region"] == "TestRegion"

    # Full hit
    assert multi_cache(2, 4) == [4, 8]
    traces = tracer.writer.pop_traces()
    assert len(traces) == 1
    spans = traces[0]
    assert len(spans) == 1
    span = spans[0]
    assert span.meta["keys"] == (
        "['tests.contrib.dogpile_cache.test_tracing:fn|2', " + "'tests.contrib.dogpile_cache.test_tracing:fn|4']"
    )
    assert span.meta["hit"] == "True"
    assert span.meta["expired"] == "False"
    assert span.meta["backend"] == "MemoryBackend"
    assert span.meta["region"] == "TestRegion"


class TestInnerFunctionCalls(object):
    def single_cache(self, x):
        return x * 2

    def multi_cache(self, *x):
        return [i * 2 for i in x]

    def test_calls_inner_functions_correctly(self, region, mocker):
        """ This ensures the get_or_create behavior of dogpile is not altered. """
        spy_single_cache = mocker.spy(self, "single_cache")
        spy_multi_cache = mocker.spy(self, "multi_cache")

        single_cache = region.cache_on_arguments()(self.single_cache)
        multi_cache = region.cache_multi_on_arguments()(self.multi_cache)

        assert 2 == single_cache(1)
        spy_single_cache.assert_called_once_with(1)

        # It's now cached - shouldn't need to call the inner function.
        spy_single_cache.reset_mock()
        assert 2 == single_cache(1)
        assert spy_single_cache.call_count == 0

        assert [6, 8] == multi_cache(3, 4)
        spy_multi_cache.assert_called_once_with(3, 4)

        # Partial hit. Only the "new" key should be passed to the inner function.
        spy_multi_cache.reset_mock()
        assert [6, 10] == multi_cache(3, 5)
        spy_multi_cache.assert_called_once_with(5)

        # Full hit. No call to inner function.
        spy_multi_cache.reset_mock()
        assert [6, 10] == multi_cache(3, 5)
        assert spy_single_cache.call_count == 0
