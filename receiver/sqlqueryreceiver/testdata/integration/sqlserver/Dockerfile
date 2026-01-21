FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:cd7c284a0dca41285f00b7abd35802b238b1875d13d2d0f66e371abb55582a9a

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]