#!/bin/bash

export PATH=/home/vagrant/go/bin:$PATH

cd /vagrant
make install-tools
make common
make docker-otelcontribcol