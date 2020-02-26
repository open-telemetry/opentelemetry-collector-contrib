import pytest

from ddtrace import _worker


def test_start():
    w = _worker.PeriodicWorkerThread()
    w.start()
    assert w.is_alive()
    w.stop()
    w.join()
    assert not w.is_alive()


def test_periodic():
    results = []

    class MyWorker(_worker.PeriodicWorkerThread):
        @staticmethod
        def run_periodic():
            results.append(object())

    w = MyWorker(interval=0, daemon=False)
    w.start()
    # results should be filled really quickly, but just in case the thread is a snail, wait
    while not results:
        pass
    w.stop()
    w.join()
    assert results


def test_on_shutdown():
    results = []

    class MyWorker(_worker.PeriodicWorkerThread):
        @staticmethod
        def on_shutdown():
            results.append(object())

    w = MyWorker()
    w.start()
    assert not results
    w.stop()
    w.join()
    assert results


def test_restart():
    w = _worker.PeriodicWorkerThread()
    w.start()
    assert w.is_alive()
    w.stop()
    w.join()
    assert not w.is_alive()

    with pytest.raises(RuntimeError):
        w.start()
