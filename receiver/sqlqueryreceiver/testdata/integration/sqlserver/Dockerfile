FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:d975fb1ea4c25c95309d7813b9f9133cde880fda16da24ab1379e277a9b2395b

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]