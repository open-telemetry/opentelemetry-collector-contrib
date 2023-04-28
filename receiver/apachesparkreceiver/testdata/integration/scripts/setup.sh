#!/bin/sh
set -e

./opt/spark/bin/spark-submit /opt/spark/examples/src/main/python/long_running.py

exit 0