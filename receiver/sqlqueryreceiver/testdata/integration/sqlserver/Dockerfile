FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:e2e5bcfe395924ff49694542191d3aefe86b6b3bd6c024f9ea01bf5a8856c56e

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]