# https://github.com/maxgio92/docker-snmpsim
FROM python:3.14-slim

RUN pip install snmpsim

RUN adduser --system --uid 1000 snmpsim

ADD data /usr/local/snmpsim/data

EXPOSE 161/udp

USER snmpsim

CMD snmpsimd.py --agent-udpv4-endpoint=0.0.0.0:161 $EXTRA_FLAGS
