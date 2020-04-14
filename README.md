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
pip install -e ./ext/opentelemetry-ext-{integration}
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

We meet weekly on Thursday, and the time of the meeting alternates between 9AM PT and 4PM PT. The meeting is subject to change depending on contributors' availability. Check the [OpenTelemetry community calendar](https://calendar.google.com/calendar/embed?src=google.com_b79e3e90j7bbsa2n2p5an5lf60%40group.calendar.google.com) for specific dates.

Meetings take place via [Zoom video conference](https://zoom.us/j/6729396170).

Meeting notes are available as a public [Google doc](https://docs.google.com/document/d/1CIMGoIOZ-c3-igzbd6_Pnxx1SjAkjwqoYSUWxPY8XIs/edit). For edit access, get in touch on [Gitter](https://gitter.im/open-telemetry/opentelemetry-python).

Approvers ([@open-telemetry/python-approvers](https://github.com/orgs/open-telemetry/teams/opentelemetry-python-contrib-approvers)):

- [Chris Kleinknecht](https://github.com/c24t), Google
- [Hector Hernandez](https://github.com/hectorhdzg), Microsoft
- [Reiley Yang](https://github.com/reyang), Microsoft

*Find more about the approver role in [community repository](https://github.com/open-telemetry/community/blob/master/community-membership.md#approver).*

Maintainers ([@open-telemetry/python-maintainers](https://github.com/orgs/open-telemetry/teams/opentelemetry-python-contrib-maintainers)):

- [Alex Boten](https://github.com/codeboten), Lightstep
- [Carlos Alberto Cortez](https://github.com/carlosalberto), Lightstep

*Find more about the maintainer role in [community repository](https://github.com/open-telemetry/community/blob/master/community-membership.md#maintainer).*

