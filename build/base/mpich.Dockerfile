FROM mpioperator/base

RUN apt update \
    && apt install -y --no-install-recommends mpich dnsutils \
    && rm -rf /var/lib/apt/lists/*

COPY mpich-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
