FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:698682bab57c02c42bc0aa274b158aeb242d8e9104149a7489628d5535805816

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]