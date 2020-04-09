# opentelemetry-auto-instr-python
The auto-instrumentation for Python (per [OTEP 0001](https://github.com/open-telemetry/oteps/blob/master/text/0001-telemetry-without-manual-instrumentation.md)) instruments each library in a separately installable package to ensure users only need to install the libraries that make sense for their use-case. This repository contains the code initially [donated by DataDog](https://www.datadoghq.com/blog/opentelemetry-instrumentation/) in the `reference` folder. All instrumentation that has been ported lives in the `instrumentation` directory.

# porting ddtrace/contrib to instrumentation

The steps below describe the process to port integrations from the reference directory containing the originally donated code to OpenTelemetry.

1. Move the code into the instrumentation directory

```
mkdir -p instrumentation/opentelemetry-instrumentation-jinja2/src/opentelemetry/instrumentation/jinja2
git mv reference/ddtrace/contrib/jinja2 instrumentation/opentelemetry-instrumentation-jinja2/src/opentelemetry/instrumentation/jinja2
```

2. Move the tests

```
git mv reference/tests/contrib/jinja2 instrumentation/opentelemetry-instrumentation-jinja2/tests
```

3. Add `README.rst`, `setup.cfg` and `setup.py` files and update them accordingly

```bash
cp _template/* instrumentation/opentelemetry-instrumentation-jinja2/
```

4. Add `version.py` file and update it accordingly

```bash
mv instrumentation/opentelemetry-instrumentation-jinja2/version.py instrumentation/opentelemetry-instrumentation-jinja2/src/opentelemetry/instrumentation/jinja2/version.py
```

5. Fix relative import paths to using ddtrace package instead of using relative paths
6. Update the code and tests to use the OpenTelemetry API
