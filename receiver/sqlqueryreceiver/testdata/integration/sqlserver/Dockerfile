FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:2c80e548ef65fb65a8ca23c3a74ed60e0964ea709f46d07c97ccaf7c437e2cc6

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]