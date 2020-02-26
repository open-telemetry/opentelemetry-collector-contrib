from ddtrace.internal.logger import DDLogger
from ddtrace.vendor.dogstatsd.base import log


def test_dogstatsd_logger():
    """Ensure dogstatsd logger is initialized as a rate limited logger"""
    assert isinstance(log, DDLogger)
