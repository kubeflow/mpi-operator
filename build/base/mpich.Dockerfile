ARG BASE_LABEL

FROM mpioperator/base:${BASE_LABEL}

RUN apt update \
    && apt install -y --no-install-recommends \
        dnsutils \
        mpich \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
