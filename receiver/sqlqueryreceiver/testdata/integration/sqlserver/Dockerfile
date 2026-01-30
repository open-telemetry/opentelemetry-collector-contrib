FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:54b23ca766287dab5f6f55162923325f07cdec6ccb42108f37c55c87e7688ebd

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]