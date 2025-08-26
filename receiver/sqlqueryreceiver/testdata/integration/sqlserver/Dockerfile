FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:2fa59c23272a23dfd9600abf4ee52c0de6ae7ac640f14c617bc717ec139a5295

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]