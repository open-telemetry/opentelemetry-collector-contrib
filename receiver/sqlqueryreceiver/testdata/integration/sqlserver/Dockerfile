FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:3bbdb7bde2126135d845868dc524008b85599993d6f262b8d9c62e82c6ef1139

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]