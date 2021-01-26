# opentelemetry-python-contrib
[![Gitter chat](https://img.shields.io/gitter/room/opentelemetry/opentelemetry-python)](https://gitter.im/open-telemetry/opentelemetry-python)[![Build status](https://travis-ci.org/open-telemetry/opentelemetry-python-contrib.svg?branch=master)](https://travis-ci.org/open-telemetry/opentelemetry-python-contrib)

The Python auto-instrumentation libraries for [OpenTelemetry](https://opentelemetry.io/) (per [OTEP 0001](https://github.com/open-telemetry/oteps/blob/master/text/0001-telemetry-without-manual-instrumentation.md))

### Installation

This repository includes installable packages for each instrumented library. Libraries that produce telemetry data should only depend on `opentelemetry-api`,
and defer the choice of the SDK to the application developer. Applications may
depend on `opentelemetry-sdk` or another package that implements the API.

**Please note** that these libraries are currently in _beta_, and shouldn't
generally be used in production environments.

The
[`instrumentation/`](https://github.com/open-telemetry/opentelemetry-python-contrib/tree/master/instrumentation)
directory includes OpenTelemetry instrumentation packages, which can be installed
separately as:

```sh
pip install opentelemetry-instrumentation-{integration}
```

To install the development versions of these packages instead, clone or fork
this repo and do an [editable
install](https://pip.pypa.io/en/stable/reference/pip_install/#editable-installs):

```sh
pip install -e ./instrumentation/opentelemetry-instrumentation-{integration}
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

We meet weekly on Thursday, and the time of the meeting alternates between 9AM PT and 4PM PT. The meeting is subject to change depending on contributors' availability. Check the [OpenTelemetry community calendar](https://calendar.google.com/calendar/embed?src=google.com_b79e3e90j7bbsa2n2p5an5lf60%40group.calendar.google.com) for specific dates.

Meetings take place via [Zoom video conference](https://zoom.us/j/6729396170). The passcode is _77777_.

Meeting notes are available as a public [Google doc](https://docs.google.com/document/d/1CIMGoIOZ-c3-igzbd6_Pnxx1SjAkjwqoYSUWxPY8XIs/edit). For edit access, get in touch on [Gitter](https://gitter.im/open-telemetry/opentelemetry-python).

Approvers ([@open-telemetry/python-approvers](https://github.com/orgs/open-telemetry/teams/python-approvers)):

- [Aaron Abbott](https://github.com/aabmass), Google
- [Diego Hurtado](https://github.com/ocelotl)
- [Hector Hernandez](https://github.com/hectorhdzg), Microsoft
- [Owais Lone](https://github.com/owais), Splunk
- [Yusuke Tsutsumi](https://github.com/toumorokoshi), Google

*Find more about the approver role in [community repository](https://github.com/open-telemetry/community/blob/master/community-membership.md#approver).*

Maintainers ([@open-telemetry/python-maintainers](https://github.com/orgs/open-telemetry/teams/python-maintainers)):

- [Alex Boten](https://github.com/codeboten), Lightstep
- [Leighton Chen](https://github.com/lzchen), Microsoft

*Find more about the maintainer role in [community repository](https://github.com/open-telemetry/community/blob/master/community-membership.md#maintainer).*

## Running Tests Locally

1. Go to your Contrib repo directory. `cd ~/git/opentelemetry-python-contrib`.
2. Create a virtual env in your Contrib repo directory. `python3 -m venv my_test_venv`.
3. Activate your virtual env. `source my_test_venv/bin/activate`.
4. Clone the [OpenTelemetry Python](https://github.com/open-telemetry/opentelemetry-python) Python Core repo to a folder named `opentelemetry-python-core`. `git clone git@github.com:open-telemetry/opentelemetry-python.git opentelemetry-python-core`.
5. Change directory to the repo that was just cloned. `cd opentelemetry-python-core`.
6. Move the head of this repo to the hash you want your tests to use. This is currently the SHA `47483865854c7adae7455f8441dab7f814f4ce2a` as seen in `.github/workflows/test.yml`. Use `git fetch && git checkout 47483865854c7adae7455f8441dab7f814f4ce2a`.
7. Go back to the root directory. `cd ../`.
8. Make sure you have `tox` installed. `pip install tox`.
9. Run tests for a package. (e.g. `tox -e test-instrumentation-flask`.)

### Thanks to all the people who already contributed!

<a href="https://github.com/open-telemetry/opentelemetry-python-contrib/graphs/contributors">
  <img src="https://contributors-img.web.app/image?repo=open-telemetry/opentelemetry-python-contrib" />
</a>

