import pytest

from ddtrace.utils import time


def test_no_states():
    watch = time.StopWatch()
    with pytest.raises(RuntimeError):
        watch.stop()


def test_start_stop():
    watch = time.StopWatch()
    watch.start()
    watch.stop()


def test_start_stop_elapsed():
    watch = time.StopWatch()
    watch.start()
    watch.stop()
    e = watch.elapsed()
    assert e > 0
    watch.start()
    assert watch.elapsed() != e


def test_no_elapsed():
    watch = time.StopWatch()
    with pytest.raises(RuntimeError):
        watch.elapsed()


def test_elapsed():
    watch = time.StopWatch()
    watch.start()
    watch.stop()
    assert watch.elapsed() > 0


def test_context_manager():
    with time.StopWatch() as watch:
        pass
    assert watch.elapsed() > 0
