FROM mcr.microsoft.com/mssql/server:2025-latest@sha256:5ffdffd2ba852051c2fc2f5b310391bf896dbaf8d16cc026a3c859d6214b30d1

ENV ACCEPT_EULA=Y
ENV SA_PASSWORD=YourStrong!Passw0rd

# Set the working directory
WORKDIR /usr/src/app

# Copy the scripts and set the executable permissions before copying them into the image
COPY entrypoint.sh configure-db.sh init.sql ./

ENTRYPOINT ["/bin/bash", "/usr/src/app/entrypoint.sh"]