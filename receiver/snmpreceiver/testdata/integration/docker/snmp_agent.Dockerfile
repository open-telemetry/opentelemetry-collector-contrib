# https://github.com/maxgio92/docker-snmpsim
FROM python:3.14.7-slim@sha256:cad9a2c871761c413caa6fdd6441c783451e740a48aaeba60ae62a8b53525ef6

RUN pip install snmpsim

RUN adduser --system --uid 1000 snmpsim

ADD data /usr/local/snmpsim/data

EXPOSE 161/udp

USER snmpsim

CMD snmpsimd.py --agent-udpv4-endpoint=0.0.0.0:161 $EXTRA_FLAGS
