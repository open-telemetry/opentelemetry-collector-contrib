FROM mcr.microsoft.com/mssql/server:2022-latest@sha256:147ee765ff1db3b86ce6ec05908e51fd0dab2feda5dd85b2721f28c77ca305eb

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]