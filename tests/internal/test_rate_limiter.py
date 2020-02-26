from __future__ import division
import mock

import pytest

from ddtrace.internal.rate_limiter import RateLimiter
from ddtrace.vendor import monotonic


def test_rate_limiter_init():
    limiter = RateLimiter(rate_limit=100)
    assert limiter.rate_limit == 100
    assert limiter.tokens == 100
    assert limiter.max_tokens == 100
    assert limiter.last_update <= monotonic.monotonic()


def test_rate_limiter_rate_limit_0():
    limiter = RateLimiter(rate_limit=0)
    assert limiter.rate_limit == 0
    assert limiter.tokens == 0
    assert limiter.max_tokens == 0

    now = monotonic.monotonic()
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        for i in range(10000):
            # Make sure the time is different for every check
            mock_time.return_value = now + i
            assert limiter.is_allowed() is False


def test_rate_limiter_rate_limit_negative():
    limiter = RateLimiter(rate_limit=-1)
    assert limiter.rate_limit == -1
    assert limiter.tokens == -1
    assert limiter.max_tokens == -1

    now = monotonic.monotonic()
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        for i in range(10000):
            # Make sure the time is different for every check
            mock_time.return_value = now + i
            assert limiter.is_allowed() is True


@pytest.mark.parametrize('rate_limit', [1, 10, 50, 100, 500, 1000])
def test_rate_limiter_is_allowed(rate_limit):
    limiter = RateLimiter(rate_limit=rate_limit)

    def check_limit():
        # Up to the allowed limit is allowed
        for _ in range(rate_limit):
            assert limiter.is_allowed() is True

        # Any over the limit is disallowed
        for _ in range(1000):
            assert limiter.is_allowed() is False

    # Start time
    now = monotonic.monotonic()

    # Check the limit for 5 time frames
    for i in range(5):
        with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
            # Keep the same timeframe
            mock_time.return_value = now + i

            check_limit()


def test_rate_limiter_is_allowed_large_gap():
    limiter = RateLimiter(rate_limit=100)

    # Start time
    now = monotonic.monotonic()
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        # Keep the same timeframe
        mock_time.return_value = now

        for _ in range(100):
            assert limiter.is_allowed() is True

    # Large gap before next call to `is_allowed()`
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        mock_time.return_value = now + 100

        for _ in range(100):
            assert limiter.is_allowed() is True


def test_rate_limiter_is_allowed_small_gaps():
    limiter = RateLimiter(rate_limit=100)

    # Start time
    now = monotonic.monotonic()
    gap = 1.0 / 100.0
    # Keep incrementing by a gap to keep us at our rate limit
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        for i in range(10000):
            # Keep the same timeframe
            mock_time.return_value = now + (gap * i)

            assert limiter.is_allowed() is True


def test_rate_liimter_effective_rate_rates():
    limiter = RateLimiter(rate_limit=100)

    # Static rate limit window
    starting_window = monotonic.monotonic()
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        mock_time.return_value = starting_window

        for _ in range(100):
            assert limiter.is_allowed() is True
            assert limiter.effective_rate == 1.0
            assert limiter.current_window == starting_window

        for i in range(1, 101):
            assert limiter.is_allowed() is False
            rate = 100 / (100 + i)
            assert limiter.effective_rate == rate
            assert limiter.current_window == starting_window

    prev_rate = 0.5
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        window = starting_window + 1.0
        mock_time.return_value = window

        for i in range(100):
            assert limiter.is_allowed() is True
            assert limiter.effective_rate == 0.75
            assert limiter.current_window == window

        for i in range(1, 101):
            assert limiter.is_allowed() is False
            rate = 100 / (100 + i)
            assert limiter.effective_rate == (rate + prev_rate) / 2
            assert limiter.current_window == window


def test_rate_liimter_effective_rate_starting_rate():
    limiter = RateLimiter(rate_limit=1)

    now = monotonic.monotonic()
    with mock.patch('ddtrace.vendor.monotonic.monotonic') as mock_time:
        mock_time.return_value = now

        # Default values
        assert limiter.current_window == 0
        assert limiter.prev_window_rate is None

        # Accessing the effective rate doesn't change anything
        assert limiter.effective_rate == 1.0
        assert limiter.current_window == 0
        assert limiter.prev_window_rate is None

        # Calling `.is_allowed()` updates the values
        assert limiter.is_allowed() is True
        assert limiter.effective_rate == 1.0
        assert limiter.current_window == now
        assert limiter.prev_window_rate is None

        # Gap of 0.9999 seconds, same window
        mock_time.return_value = now + 0.9999
        assert limiter.is_allowed() is False
        # DEV: We have rate_limit=1 set
        assert limiter.effective_rate == 0.5
        assert limiter.current_window == now
        assert limiter.prev_window_rate is None

        # Gap of 1.0 seconds, new window
        mock_time.return_value = now + 1.0
        assert limiter.is_allowed() is True
        assert limiter.effective_rate == 0.75
        assert limiter.current_window == (now + 1.0)
        assert limiter.prev_window_rate == 0.5

        # Gap of 1.9999 seconds, same window
        mock_time.return_value = now + 1.9999
        assert limiter.is_allowed() is False
        assert limiter.effective_rate == 0.5
        assert limiter.current_window == (now + 1.0)  # Same as old window
        assert limiter.prev_window_rate == 0.5

        # Large gap of 100 seconds, new window
        mock_time.return_value = now + 100.0
        assert limiter.is_allowed() is True
        assert limiter.effective_rate == 0.75
        assert limiter.current_window == (now + 100.0)
        assert limiter.prev_window_rate == 0.5
