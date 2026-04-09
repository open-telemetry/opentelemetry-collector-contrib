FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:ca62ad2d75975074cbc9ac39a1329cd88af9cd3ea2ffb59d6302579a97cf32f4

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]