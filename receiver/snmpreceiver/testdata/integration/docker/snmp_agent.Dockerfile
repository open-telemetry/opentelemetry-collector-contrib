# https://github.com/maxgio92/docker-snmpsim
# WARNING: This can't be updated beyond 3.5 as snmpsim seems to have issues running on linux!
FROM python:3.5-slim

RUN pip install snmpsim

RUN adduser --system --uid 1000 snmpsim

ADD data /usr/local/snmpsim/data

EXPOSE 1024/udp

USER snmpsim

CMD snmpsimd.py --agent-udpv4-endpoint=0.0.0.0:1024 $EXTRA_FLAGS
