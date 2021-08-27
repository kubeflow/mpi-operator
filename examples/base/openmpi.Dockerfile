FROM mpioperator/base

RUN apt update \
    && apt install -y --no-install-recommends openmpi-bin \
    && rm -rf /var/lib/apt/lists/*
