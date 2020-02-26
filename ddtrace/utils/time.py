from ..vendor import monotonic


class StopWatch(object):
    """A simple timer/stopwatch helper class.

    Not thread-safe (when a single watch is mutated by multiple threads at
    the same time). Thread-safe when used by a single thread (not shared) or
    when operations are performed in a thread-safe manner on these objects by
    wrapping those operations with locks.

    It will use the `monotonic`_ pypi library to find an appropriate
    monotonically increasing time providing function (which typically varies
    depending on operating system and Python version).

    .. _monotonic: https://pypi.python.org/pypi/monotonic/
    """

    def __init__(self):
        self._started_at = None
        self._stopped_at = None

    def start(self):
        """Starts the watch."""
        self._started_at = monotonic.monotonic()
        return self

    def elapsed(self):
        """Get how many seconds have elapsed.

        :return: Number of seconds elapsed
        :rtype: float
        """
        # NOTE: datetime.timedelta does not support nanoseconds, so keep a float here
        if self._started_at is None:
            raise RuntimeError("Can not get the elapsed time of a stopwatch" " if it has not been started/stopped")
        if self._stopped_at is None:
            now = monotonic.monotonic()
        else:
            now = self._stopped_at
        return now - self._started_at

    def __enter__(self):
        """Starts the watch."""
        self.start()
        return self

    def __exit__(self, tp, value, traceback):
        """Stops the watch."""
        self.stop()

    def stop(self):
        """Stops the watch."""
        if self._started_at is None:
            raise RuntimeError("Can not stop a stopwatch that has not been" " started")
        self._stopped_at = monotonic.monotonic()
        return self
