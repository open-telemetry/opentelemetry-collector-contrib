FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:60dc610b741e278c027f584c2f054693a6db481c06e03c985dc746d41403e375

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]