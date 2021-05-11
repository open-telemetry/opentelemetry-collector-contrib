FROM cassandra:3.11
ENV LOCAL_JMX=no
ENV JVM_OPTS="-Djava.rmi.server.hostname=0.0.0.0"
RUN echo 'cassandra cassandra' > /etc/cassandra/jmxremote.password
RUN chmod 0400 /etc/cassandra/jmxremote.password
