FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:320daf857745ef0092c6e53e82d0bf81ed6bca2794e6ccbffb72cf6a8d14a441

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]