==============
 Contributing
==============

When contributing to this repository, we advise you to discuss the change you
wish to make via an `issue <https://github.com/DataDog/dd-trace-py/issues>`_.

Branches
========

Developement happens in the `master` branch. When all the features for the next
milestone are merged, the next version is released and tagged on the `master`
branch as `vVERSION`.

Your pull request should targets the `master` branch.

Once a new version is released, a `release/VERSION` branch might be created to
support micro releases to `VERSION`. Patches should be cherry-picking from the
`master` branch where possible — or otherwise created from scratch.


Pull Request Process
====================

In order to be merged, a pull request needs to meet the following
conditions:

1. The test suite must pass.
2. One of the repository Members must approve the pull request.
3. Proper unit and integration testing must be implemented.
4. Proper documentation must be written.

Splitting Pull Requests
=======================

If you discussed your feature within an issue (as advised), there's a great
chance that the implementation appears doable in several steps. In order to
facilite the review process, we strongly advise to split your feature
implementation in small pull requests (if that is possible) so they contain a
very small number of commits (a single commit per pull request being optimal).

That ensures that:

1. Each commit passes the test suite.
2. The code reviewing process done by humans is easier as there is less code to
   understand at a glance.

Internal API
============

The `ddtrace.internal` module contains code that must only be used inside
`ddtrace` itself. Relying on the API of this module is dangerous and can break
at anytime. Don't do it.

Python Versions and Implementations Support
===========================================

The following Python implementations are supported:

- CPython

Versions of those implementations that are supported are the Python versions
that are currently supported by the community.

Libraries Support
=================

External libraries support is implemented in submodules of the `ddtest.contrib`
module.

Our goal is to support:

- The latest version of a library.
- All versions of a library that have been released less than 1 year ago.

Support for older versions of a library will be kept as long as possible as
long as it can be done without too much pain and backward compatibility — on a
best effort basis. Therefore, support for old versions of a library might be
dropped from the testing pipeline at anytime.

Code Style
==========

The code style is enforced by `flake8 <https://pypi.org/project/flake8>`_, its
configuration, and possibly extensions. No code style review should be done by
a human. All code style enforcement must be automatized to avoid bikeshedding
and losing time.
