---
<p align="center">
  <strong>
    <a href="https://opentelemetry.io/docs/instrumentation/python/getting-started/">Getting Started<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://opentelemetry-python-contrib.readthedocs.io/">API Documentation<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-python/discussions">Getting In Touch (GitHub Discussions)<a/>
  </strong>
</p>

<p align="center">
  <a href="https://github.com/open-telemetry/opentelemetry-python-contrib/releases">
    <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/open-telemetry/opentelemetry-python-contrib?include_prereleases&style=for-the-badge">
  </a>
  <a href="https://codecov.io/gh/open-telemetry/opentelemetry-python-contrib/branch/main/">
    <img alt="Codecov Status" src="https://img.shields.io/codecov/c/github/open-telemetry/opentelemetry-python-contrib?style=for-the-badge">
  </a>
  <a href="https://github.com/open-telemetry/opentelemetry-python-contrib/blob/main/LICENSE">
    <img alt="license" src="https://img.shields.io/badge/license-Apache_2.0-green.svg?style=for-the-badge">
  </a>
  <br/>
  <a href="https://github.com/open-telemetry/opentelemetry-python-contrib/actions?query=workflow%3ATest+branch%3Amaster">
    <img alt="Build Status" src="https://github.com/open-telemetry/opentelemetry-python-contrib/workflows/Test/badge.svg">
  </a>
  <img alt="Beta" src="https://img.shields.io/badge/status-beta-informational?logo=data:image/png;base64,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">
</p>

<p align="center">
  <strong>
    <a href="CONTRIBUTING.md">Contributing<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://opentelemetry-python-contrib.readthedocs.io/en/stable/#examples">Examples<a/>
  </strong>
</p>

---

## OpenTelemetry Python Contrib

The Python auto-instrumentation libraries for [OpenTelemetry](https://opentelemetry.io/) (per [OTEP 0001](https://github.com/open-telemetry/oteps/blob/main/text/0001-telemetry-without-manual-instrumentation.md))

### Installation

This repository includes installable packages for each instrumented library. Libraries that produce telemetry data should only depend on `opentelemetry-api`,
and defer the choice of the SDK to the application developer. Applications may
depend on `opentelemetry-sdk` or another package that implements the API.

**Please note** that these libraries are currently in _beta_, and shouldn't
generally be used in production environments.

The
[`instrumentation/`](https://github.com/open-telemetry/opentelemetry-python-contrib/tree/main/instrumentation)
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

## Releasing

Maintainers release new versions of the packages in `opentelemetry-python-contrib` on a monthly cadence. See [releases](https://github.com/open-telemetry/opentelemetry-python-contrib/releases) for all previous releases.

Contributions that enhance OTel for Python are welcome to be hosted upstream for the benefit of group collaboration. Maintainers will look for things like good documentation, good unit tests, and in general their own confidence when deciding to release a package with the stability guarantees that are implied with a `1.0` release.

To resolve this, members of the community are encouraged to commit to becoming a CODEOWNER for packages in `-contrib` that they feel experienced enough to maintain. CODEOWNERS can then follow the checklist below to release `-contrib` packages as 1.0 stable:

### Releasing a package as `1.0` stable

To release a package as `1.0` stable, the package:
- SHOULD have a CODEOWNER. To become one, submit an issue and explain why you meet the responsibilities found in [CODEOWNERS](.github/CODEOWNERS).
- MUST have unit tests that cover all supported versions of the instrumented library.
  - e.g. Instrumentation packages might use different techniques to instrument different major versions of python packages
- MUST have clear documentation for non-obvious usages of the package
  - e.g. If an instrumentation package uses flags, a token as context, or parameters that are not typical of the `BaseInstrumentor` class, these are documented
- After the release of `1.0`, a CODEOWNER may no longer feel like they have the bandwidth to meet the responsibilities of maintaining the package. That's not a problem at all, life happens! However, if that is the case, we ask that the CODEOWNER please raise an issue indicating that they would like to be removed as a CODEOWNER so that they don't get pinged on future PRs. Ultimately, we hope to use that issue to find a new CODEOWNER.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

We meet weekly on Thursday, and the time of the meeting alternates between 9AM PT and 4PM PT. The meeting is subject to change depending on contributors' availability. Check the [OpenTelemetry community calendar](https://calendar.google.com/calendar/embed?src=google.com_b79e3e90j7bbsa2n2p5an5lf60%40group.calendar.google.com) for specific dates and for the Zoom link.

Meeting notes are available as a public [Google doc](https://docs.google.com/document/d/1CIMGoIOZ-c3-igzbd6_Pnxx1SjAkjwqoYSUWxPY8XIs/edit). For edit access, get in touch on [GitHub Discussions](https://github.com/open-telemetry/opentelemetry-python/discussions).

Approvers ([@open-telemetry/python-approvers](https://github.com/orgs/open-telemetry/teams/python-approvers)):

- [Aaron Abbott](https://github.com/aabmass), Google
- [Alex Boten](https://github.com/codeboten), Lightstep
- [Nathaniel Ruiz Nowell](https://github.com/NathanielRN), AWS
- [Owais Lone](https://github.com/owais), Splunk

*Find more about the approver role in [community repository](https://github.com/open-telemetry/community/blob/main/community-membership.md#approver).*

Maintainers ([@open-telemetry/python-maintainers](https://github.com/orgs/open-telemetry/teams/python-maintainers)):

- [Diego Hurtado](https://github.com/ocelotl), Lightstep
- [Leighton Chen](https://github.com/lzchen), Microsoft
- [Srikanth Chekuri](https://github.com/srikanthccv)

*Find more about the maintainer role in [community repository](https://github.com/open-telemetry/community/blob/main/community-membership.md#maintainer).*

## Running Tests Locally

1. Go to your Contrib repo directory. `cd ~/git/opentelemetry-python-contrib`.
2. Create a virtual env in your Contrib repo directory. `python3 -m venv my_test_venv`.
3. Activate your virtual env. `source my_test_venv/bin/activate`.
4. Make sure you have `tox` installed. `pip install tox`.
5. Run tests for a package. (e.g. `tox -e test-instrumentation-flask`.)

### Thanks to all the people who already contributed!

<a href="https://github.com/open-telemetry/opentelemetry-python-contrib/graphs/contributors">
  <img src="https://contributors-img.web.app/image?repo=open-telemetry/opentelemetry-python-contrib" />
</a>

