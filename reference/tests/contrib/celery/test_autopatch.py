#!/usr/bin/env python
import subprocess
import unittest


class DdtraceRunTest(unittest.TestCase):
    """Test that celery is patched successfully if run with ddtrace-run."""

    def test_autopatch(self):
        out = subprocess.check_output(
            ['ddtrace-run', 'python', 'tests/contrib/celery/autopatch.py']
        )
        assert out.startswith(b'Test success')
