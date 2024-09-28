FROM mcr.microsoft.com/mssql/server:2017-latest

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

COPY entrypoint.sh /usr/src/app/entrypoint.sh
COPY configure-db.sh /usr/src/app/configure-db.sh
COPY init.sql /usr/src/app/init.sql

RUN chmod +x /usr/src/app/entrypoint.sh /usr/src/app/configure-db.sh

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]