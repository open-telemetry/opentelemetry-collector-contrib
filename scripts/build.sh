#!/bin/sh

# This script builds wheels for the API, SDK, and extension packages in the
# dist/ dir, to be uploaded to PyPI.

set -ev

# Get the latest versions of packaging tools
python3 -m pip install --upgrade pip setuptools wheel

BASEDIR=$(dirname $(readlink -f $(dirname $0)))
DISTDIR=dist

(
  cd $BASEDIR
  mkdir -p $DISTDIR
  rm -rf $DISTDIR/*

 for d in exporter/*/ opentelemetry-instrumentation/ opentelemetry-contrib-instrumentations/ opentelemetry-distro/ instrumentation/*/ propagator/*/ sdk-extension/*/ util/*/ ; do
   (
     echo "building $d"
     cd "$d"
     # Some ext directories (such as docker tests) are not intended to be
     # packaged. Verify the intent by looking for a setup.py.
     if [ -f setup.py ]; then
      python3 setup.py sdist --dist-dir "$BASEDIR/dist/" clean --all
     fi
   )
 done
 # Build a wheel for each source distribution
 (
   cd $DISTDIR
   for x in *.tar.gz ; do
    # NOTE: We filter beta vs 1.0 package at this point because we can read the
    # version directly from the .tar.gz file.
    if (echo "$x" | grep -Eq ^opentelemetry-.*-0\..*\.tar\.gz$); then
      pip wheel --no-deps $x
    else
      echo "Skipping $x because it is not in pre-1.0 state and should be released using a tag."
      rm $x
    fi
   done
 )
)
