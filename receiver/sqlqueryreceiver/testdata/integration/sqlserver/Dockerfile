FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:fafbdd0ced67edd292420ef8c4cc8fc17ebb4a06f56d2afa9f160a6edef96054

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]