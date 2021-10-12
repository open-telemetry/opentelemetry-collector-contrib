OpenTelemetry-Python-Contrib
============================

Complimentary instrumentation and vendor-specific packages for use with the
Python `OpenTelemetry <https://opentelemetry.io/>`_ client.

.. image:: https://img.shields.io/badge/slack-chat-green.svg
   :target: https://cloud-native.slack.com/archives/C01PD4HUVBL
   :alt: Slack Chat


**Please note** that this library is currently in _beta_, and shouldn't
generally be used in production environments.

Installation
------------

There are several complimentary packages available on PyPI which can be
installed separately via pip:

.. code-block:: sh

    pip install opentelemetry-exporter-{exporter}
    pip install opentelemetry-instrumentation-{instrumentation}
    pip install opentelemetry-sdk-extension-{sdkextension}

A complete list of packages can be found at the 
`Contrib repo instrumentation <https://github.com/open-telemetry/opentelemetry-python-contrib/tree/main/instrumentation>`_
and `Contrib repo exporter <https://github.com/open-telemetry/opentelemetry-python-contrib/tree/main/exporter>`_ directories.

Extensions
----------

Visit `OpenTelemetry Registry <https://opentelemetry.io/registry/?s=python>`_ to
find a lot of related projects like exporters, instrumentation libraries, tracer
implementations, etc.

Installing Cutting Edge Packages
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

While the project is pre-1.0, there may be significant functionality that
has not yet been released to PyPI. In that situation, you may want to
install the packages directly from the repo. This can be done by cloning the
repository and doing an `editable
install <https://pip.pypa.io/en/stable/reference/pip_install/#editable-installs>`_:

.. code-block:: sh

    git clone https://github.com/open-telemetry/opentelemetry-python-contrib.git
    cd opentelemetry-python-contrib
    pip install -e ./instrumentation/opentelemetry-instrumentation-flask
    pip install -e ./instrumentation/opentelemetry-instrumentation-botocore
    pip install -e ./sdk-extension/opentelemetry-sdk-extension-aws


.. toctree::
    :maxdepth: 2
    :caption: OpenTelemetry Exporters
    :name: exporters
    :glob:

    exporter/**

.. toctree::
    :maxdepth: 2
    :caption: OpenTelemetry Instrumentations
    :name: Instrumentations
    :glob:

    instrumentation/**

.. toctree::
    :maxdepth: 2
    :caption: OpenTelemetry Propagators
    :name: Propagators
    :glob:

    propagator/**

.. toctree::
    :maxdepth: 2
    :caption: OpenTelemetry Performance
    :name: Performance
    :glob:

    performance/**

.. toctree::
    :maxdepth: 2
    :caption: OpenTelemetry SDK Extensions
    :name: SDK Extensions
    :glob:

    sdk-extension/**

Indices and tables
------------------

* :ref:`genindex`
* :ref:`modindex`
* :ref:`search`
