FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:cee0f4db03b5192de777d37a22455949d1c2c230bb48c0a732b88b0933117a72

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]