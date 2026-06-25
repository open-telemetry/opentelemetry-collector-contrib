FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:2cd0aec4a3bfc3cf9205bed3f7922f4c6208f7c767dc62edcee308d0fd7d56d0

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]