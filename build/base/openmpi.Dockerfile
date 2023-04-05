ARG BASE_LABEL

FROM mpioperator/base:${BASE_LABEL}

RUN apt update \
    && apt install -y --no-install-recommends openmpi-bin \
    && rm -rf /var/lib/apt/lists/*
