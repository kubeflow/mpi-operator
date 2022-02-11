FROM bash AS downloader

RUN wget https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB -O key.PUB

FROM mpioperator/base

COPY --from=downloader key.PUB /tmp/key.PUB

# Install Intel oneAPI keys.
RUN apt update \
    && apt install -y --no-install-recommends gnupg2 ca-certificates \
    && apt-key add /tmp/key.PUB \
    && rm /tmp/key.PUB \
    && echo "deb https://apt.repos.intel.com/oneapi all main" | tee /etc/apt/sources.list.d/oneAPI.list \
    && apt remove -y gnupg2 ca-certificates \
    && apt autoremove -y \
    && apt update \
    && apt install -y --no-install-recommends \
        dnsutils \
        intel-oneapi-mpi \
    && rm -rf /var/lib/apt/lists/*

COPY intel-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
