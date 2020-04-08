try:
    # detect if concurrent.futures is available as a Python
    # stdlib or Python 2.7 backport
    from ..futures import patch as wrap_futures, unpatch as unwrap_futures
    futures_available = True
except ImportError:
    def wrap_futures():
        pass

    def unwrap_futures():
        pass
    futures_available = False
