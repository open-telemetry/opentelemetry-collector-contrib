#!/bin/sh

set -x

export DEBIAN_FRONTEND=noninteractive 
apt-get update
apt-get -y upgrade
apt-get -y install apt-transport-https
apt-get install --yes docker.io docker-compose

add-apt-repository ppa:longsleep/golang-backports
apt update
apt install -y golang-go

usermod -aG docker vagrant
systemctl start docker

su vagrant -c '/vagrant/vagrant/build.sh'