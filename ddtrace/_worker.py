import atexit
import threading
import os

from .internal.logger import get_logger

_LOG = get_logger(__name__)


class PeriodicWorkerThread(object):
    """Periodic worker thread.

    This class can be used to instantiate a worker thread that will run its `run_periodic` function every `interval`
    seconds.

    The method `on_shutdown` will be called on worker shutdown. The worker will be shutdown when the program exits and
    can be waited for with the `exit_timeout` parameter.

    """

    _DEFAULT_INTERVAL = 1.0

    def __init__(self, interval=_DEFAULT_INTERVAL, exit_timeout=None, name=None, daemon=True):
        """Create a new worker thread that runs a function periodically.

        :param interval: The interval in seconds to wait between calls to `run_periodic`.
        :param exit_timeout: The timeout to use when exiting the program and waiting for the thread to finish.
        :param name: Name of the worker.
        :param daemon: Whether the worker should be a daemon.
        """

        self._thread = threading.Thread(target=self._target, name=name)
        self._thread.daemon = daemon
        self._stop = threading.Event()
        self.interval = interval
        self.exit_timeout = exit_timeout
        atexit.register(self._atexit)

    def _atexit(self):
        self.stop()
        if self.exit_timeout is not None:
            key = 'ctrl-break' if os.name == 'nt' else 'ctrl-c'
            _LOG.debug(
                'Waiting %d seconds for %s to finish. Hit %s to quit.',
                self.exit_timeout, self._thread.name, key,
            )
            self.join(self.exit_timeout)

    def start(self):
        """Start the periodic worker."""
        _LOG.debug('Starting %s thread', self._thread.name)
        self._thread.start()

    def stop(self):
        """Stop the worker."""
        _LOG.debug('Stopping %s thread', self._thread.name)
        self._stop.set()

    def is_alive(self):
        return self._thread.is_alive()

    def join(self, timeout=None):
        return self._thread.join(timeout)

    def _target(self):
        while not self._stop.wait(self.interval):
            self.run_periodic()
        self._on_shutdown()

    @staticmethod
    def run_periodic():
        """Method executed every interval."""
        pass

    def _on_shutdown(self):
        _LOG.debug('Shutting down %s thread', self._thread.name)
        self.on_shutdown()

    @staticmethod
    def on_shutdown():
        """Method ran on worker shutdown."""
        pass
