# Contributing to opentelemetry-python-contrib

The Python special interest group (SIG) meets regularly. See the OpenTelemetry
[community](https://github.com/open-telemetry/community#python-sdk) repo for
information on this and other language SIGs.

See the [public meeting notes](https://docs.google.com/document/d/1CIMGoIOZ-c3-igzbd6_Pnxx1SjAkjwqoYSUWxPY8XIs/edit)
for a summary description of past meetings. To request edit access, join the
meeting or get in touch on [Slack](https://cloud-native.slack.com/archives/C01PD4HUVBL).

See to the [community membership document](https://github.com/open-telemetry/community/blob/main/community-membership.md)
on how to become a [**Member**](https://github.com/open-telemetry/community/blob/main/community-membership.md#member),
[**Approver**](https://github.com/open-telemetry/community/blob/main/community-membership.md#approver)
and [**Maintainer**](https://github.com/open-telemetry/community/blob/main/community-membership.md#maintainer).

## Find a Buddy and get Started Quickly!

If you are looking for someone to help you find a starting point and be a resource for your first contribution, join our
Slack and find a buddy!

1. Join [Slack](https://slack.cncf.io/) and join our [chat room](https://cloud-native.slack.com/archives/C01PD4HUVBL).
2. Post in the room with an introduction to yourself, what area you are interested in (check issues marked "Help Wanted"),
and say you are looking for a buddy. We will match you with someone who has experience in that area.

Your OpenTelemetry buddy is your resource to talk to directly on all aspects of contributing to OpenTelemetry: providing
context, reviewing PRs, and helping those get merged. Buddies will not be available 24/7, but is committed to responding during their normal contribution hours.

## Development

This project uses [tox](https://tox.readthedocs.io) to automate
some aspects of development, including testing against multiple Python versions.
To install `tox`, run:

```console
$ pip install tox
```

You can run `tox` with the following arguments:

- `tox` to run all existing tox commands, including unit tests for all packages
  under multiple Python versions
- `tox -e docs` to regenerate the API docs
- `tox -e py37-test-instrumentation-aiopg` to e.g. run the aiopg instrumentation unit tests under a specific
  Python version
- `tox -e spellcheck` to run a spellcheck on all the code
- `tox -e lint` to run lint checks on all code

`black` and `isort` are executed when `tox -e lint` is run. The reported errors can be tedious to fix manually.
An easier way to do so is:

1. Run `.tox/lint/bin/black .`
2. Run `.tox/lint/bin/isort .`

See
[`tox.ini`](https://github.com/open-telemetry/opentelemetry-python/blob/main/tox.ini)
for more detail on available tox commands.

### Benchmarks

Performance progression of benchmarks for packages distributed by OpenTelemetry Python can be viewed as a [graph of throughput vs commit history](https://opentelemetry-python-contrib.readthedocs.io/en/latest/performance/benchmarks.html). From the linked page, you can download a JSON file with the performance results.

Running the `tox` tests also runs the performance tests if any are available. Benchmarking tests are done with `pytest-benchmark` and they output a table with results to the console.

To write benchmarks, simply use the [pytest benchmark fixture](https://pytest-benchmark.readthedocs.io/en/latest/usage.html#usage) like the following:

```python
def test_simple_start_span(benchmark):
    def benchmark_start_as_current_span(span_name, attribute_num):
        span = tracer.start_span(
            span_name,
            attributes={"count": attribute_num},
        )
        span.end()

    benchmark(benchmark_start_as_current_span, "benchmarkedSpan", 42)
```

Make sure the test file is under the `tests/performance/benchmarks/` folder of
the package it is benchmarking and further has a path that corresponds to the
file in the package it is testing. Make sure that the file name begins with
`test_benchmark_`. (e.g. `propagator/opentelemetry-propagator-aws-xray/tests/performance/benchmarks/trace/propagation/test_benchmark_aws_xray_propagator.py`)

## Pull Requests

### How to Send Pull Requests

Everyone is welcome to contribute code to `opentelemetry-python-contrib` via GitHub
pull requests (PRs).

To create a new PR, fork the project in GitHub and clone the upstream repo:

```sh
$ git clone https://github.com/open-telemetry/opentelemetry-python-contrib.git
```

Add your fork as an origin:

```sh
$ git remote add fork https://github.com/YOUR_GITHUB_USERNAME/opentelemetry-python-contrib.git
```

Run tests:

```sh
# make sure you have all supported versions of Python installed
$ pip install tox  # only first time.
$ tox  # execute in the root of the repository
```

Check out a new branch, make modifications and push the branch to your fork:

```sh
$ git checkout -b feature
# edit files
$ git commit
$ git push fork feature
```

Open a pull request against the main `opentelemetry-python-contrib` repo.

### How to Receive Comments

* If the PR is not ready for review, please put `[WIP]` in the title, tag it
  as `work-in-progress`, or mark it as [`draft`](https://github.blog/2019-02-14-introducing-draft-pull-requests/).
* Make sure CLA is signed and CI is clear.

### How to Get PRs Merged

A PR is considered to be **ready to merge** when:
* It has received two approvals from [Approvers](https://github.com/open-telemetry/community/blob/main/community-membership.md#approver)
  / [Maintainers](https://github.com/open-telemetry/community/blob/main/community-membership.md#maintainer)
  (at different companies).
* Major feedbacks are resolved.
* It has been open for review for at least one working day. This gives people
  reasonable time to review.
* Trivial change (typo, cosmetic, doc, etc.) doesn't have to wait for one day.
* Urgent fix can take exception as long as it has been actively communicated.
* A changelog entry is added to the corresponding changelog for the code base, if there is any impact on behavior. e.g. doc entries are not required, but small bug entries are.

Any Approver / Maintainer can merge the PR once it is **ready to merge**.

## Design Choices

As with other OpenTelemetry clients, opentelemetry-python follows the
[opentelemetry-specification](https://github.com/open-telemetry/opentelemetry-specification).

It's especially valuable to read through the [library guidelines](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/library-guidelines.md).

### Focus on Capabilities, Not Structure Compliance

OpenTelemetry is an evolving specification, one where the desires and
use cases are clear, but the method to satisfy those uses cases are not.

As such, contributions should provide functionality and behavior that
conforms to the specification, but the interface and structure is flexible.

It is preferable to have contributions follow the idioms of the language
rather than conform to specific API names or argument patterns in the spec.

For a deeper discussion, see: https://github.com/open-telemetry/opentelemetry-specification/issues/165

## Running Tests Locally

1. Go to your Contrib repo directory. `git clone git@github.com:open-telemetry/opentelemetry-python-contrib.git && cd opentelemetry-python-contrib`.
2. Make sure you have `tox` installed. `pip install tox`.
3. Run `tox` without any arguments to run tests for all the packages. Read more about [tox](https://tox.readthedocs.io/en/latest/).

### Testing against a different Core repo branch/commit

Some of the tox targets install packages from the [OpenTelemetry Python Core Repository](https://github.com/open-telemetry/opentelemetry-python) via pip. The version of the packages installed defaults to the main branch in that repository when tox is run locally. It is possible to install packages tagged with a specific git commit hash by setting an environment variable before running tox as per the following example:

CORE_REPO_SHA=c49ad57bfe35cfc69bfa863d74058ca9bec55fc3 tox

The continuation integration overrides that environment variable with as per the configuration [here](https://github.com/open-telemetry/opentelemetry-python-contrib/blob/main/.github/workflows/test.yml#L9).


## Style Guide

* docstrings should adhere to the [Google Python Style
  Guide](http://google.github.io/styleguide/pyguide.html#38-comments-and-docstrings)
  as specified with the [napoleon
  extension](http://www.sphinx-doc.org/en/master/usage/extensions/napoleon.html#google-vs-numpy)
  extension in [Sphinx](http://www.sphinx-doc.org/en/master/index.html).

## Guideline for instrumentations

Below is a checklist of things to be mindful of when implementing a new instrumentation or working on a specific instrumentation. It is one of our goals as a community to keep the implementation specific details of instrumentations as similar across the board as possible for ease of testing and feature parity. It is also good to abstract as much common functionality as possible.

- Follow semantic conventions
  - The instrumentation should follow the semantic conventions defined [here](https://github.com/open-telemetry/opentelemetry-specification/tree/main/semantic_conventions)
- Extends from [BaseInstrumentor](https://github.com/open-telemetry/opentelemetry-python-contrib/blob/main/opentelemetry-instrumentation/src/opentelemetry/instrumentation/instrumentor.py#L26)
- Supports auto-instrumentation
  - Add an entry point (ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/f045c43affff6ff1af8fa2f7514a4fdaca97dacf/instrumentation/opentelemetry-instrumentation-requests/pyproject.toml#L44)
  - Run `python scripts/generate_instrumentation_bootstrap.py` after adding a new instrumentation package.
- Functionality that is common amongst other instrumentation and can be abstracted [here](https://github.com/open-telemetry/opentelemetry-python-contrib/tree/main/opentelemetry-instrumentation/src/opentelemetry/instrumentation)
- Request/response [hooks](https://github.com/open-telemetry/opentelemetry-python-contrib/issues/408) for http instrumentations
- `suppress_instrumentation` functionality
  - ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/3ec77360cb20482b08b30312a6bedc8b946e3fa1/instrumentation/opentelemetry-instrumentation-requests/src/opentelemetry/instrumentation/requests/__init__.py#L111
- Suppress propagation functionality
  - https://github.com/open-telemetry/opentelemetry-python-contrib/issues/344 for more context
- `exclude_urls` functionality
  - ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/0fcb60d2ad139f78a52edd85b1cc4e32f2e962d0/instrumentation/opentelemetry-instrumentation-flask/src/opentelemetry/instrumentation/flask/__init__.py#L91
- `url_filter` functionality
  - ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/0fcb60d2ad139f78a52edd85b1cc4e32f2e962d0/instrumentation/opentelemetry-instrumentation-aiohttp-client/src/opentelemetry/instrumentation/aiohttp_client/__init__.py#L235
- `is_recording()` optimization on non-sampled spans
  - ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/main/instrumentation/opentelemetry-instrumentation-requests/src/opentelemetry/instrumentation/requests/__init__.py#L133
- Appropriate error handling
  - ex. https://github.com/open-telemetry/opentelemetry-python-contrib/blob/main/instrumentation/opentelemetry-instrumentation-requests/src/opentelemetry/instrumentation/requests/__init__.py#L146


## Expectations from contributors

OpenTelemetry is an open source community, and as such, greatly encourages contributions from anyone interested in the project. With that being said, there is a certain level of expectation from contributors even after a pull request is merged, specifically pertaining to instrumentations. The OpenTelemetry Python community expects contributors to maintain a level of support and interest in the instrumentations they contribute. This is to ensure that the instrumentation does not become stale and still functions the way the original contributor intended. Some instrumentations also pertain to libraries that the current members of the community are not so familiar with, so it is necessary to rely on the expertise of the original contributing parties.

