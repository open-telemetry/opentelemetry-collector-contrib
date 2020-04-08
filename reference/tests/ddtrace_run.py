import os
import sys

# DEV: We must append to sys path before importing ddtrace_run
sys.path.append('.')
from ddtrace.commands import ddtrace_run  # noqa

os.environ['PYTHONPATH'] = '{}:{}'.format(os.getenv('PYTHONPATH'), os.path.abspath('.'))
ddtrace_run.main()
