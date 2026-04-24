FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:a93989eebfb50ae37ee0bfecfaf03648421859558eef6cbff9dec0db3976f192

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]