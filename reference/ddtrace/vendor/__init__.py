"""
ddtrace.vendor
==============
Install vendored dependencies under a different top level package to avoid importing `ddtrace/__init__.py`
whenever a dependency is imported. Doing this allows us to have a little more control over import order.


Dependencies
============

msgpack
-------

Website: https://msgpack.org/
Source: https://github.com/msgpack/msgpack-python
Version: 0.6.1
License: Apache License, Version 2.0

Notes:
  If you need to update any `*.pyx` files, be sure to run `cython --cplus msgpack/_cmsgpack.pyx` to regenerate `_cmsgpack.cpp`

  `_packer.pyx` and `_unpacker.pyx` were updated to import from `ddtrace.vendor.msgpack`

six
---

Website: https://six.readthedocs.io/
Source: https://github.com/benjaminp/six
Version: 1.11.0
License: MIT

Notes:
  `six/__init__.py` is just the source code's `six.py`
  `curl https://raw.githubusercontent.com/benjaminp/six/1.11.0/six.py > ddtrace/vendor/six/__init__.py`


wrapt
-----

Website: https://wrapt.readthedocs.io/en/latest/
Source: https://github.com/GrahamDumpleton/wrapt/
Version: 1.11.1
License: BSD 2-Clause "Simplified" License

Notes:
  `wrapt/__init__.py` was updated to include a copy of `wrapt`'s license: https://github.com/GrahamDumpleton/wrapt/blob/1.11.1/LICENSE

  `setup.py` will attempt to build the `wrapt/_wrappers.c` C module

dogstatsd
---------

Website: https://datadogpy.readthedocs.io/en/latest/
Source: https://github.com/DataDog/datadogpy
Version: 0.28.0
License: Copyright (c) 2015, Datadog <info@datadoghq.com>

Notes:
  `dogstatsd/__init__.py` was updated to include a copy of the `datadogpy` license: https://github.com/DataDog/datadogpy/blob/master/LICENSE
  Only `datadog.dogstatsd` module was vendored to avoid unnecessary dependencies
  `datadog/util/compat.py` was copied to `dogstatsd/compat.py`

monotonic
---------

Website: https://pypi.org/project/monotonic/
Source: https://github.com/atdt/monotonic
Version: 1.5
License: Apache License 2.0

Notes:
  The source `monotonic.py` was added as `monotonic/__init__.py`

  No other changes were made

debtcollector
-------------

Website: https://docs.openstack.org/debtcollector/latest/index.html
Source: https://github.com/openstack/debtcollector
Version: 1.22.0
License: Apache License 2.0

Notes:
   Removed dependency on `pbr` and manually set `__version__`

"""

# Initialize `ddtrace.vendor.datadog.base.log` logger with our custom rate limited logger
# DEV: This helps ensure if there are connection issues we do not spam their logs
# DEV: Overwrite `base.log` instead of `get_logger('datadog.dogstatsd')` so we do
#      not conflict with any non-vendored datadog.dogstatsd logger
from ..internal.logger import get_logger
from .dogstatsd import base
base.log = get_logger('ddtrace.vendor.dogstatsd')
