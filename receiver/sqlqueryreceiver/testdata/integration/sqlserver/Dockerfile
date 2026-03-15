FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:d24e4c2ba946e91ef4065fc2a2e43bc4cd3fadf1bd6feac4f5d8e4185aa98cac

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]