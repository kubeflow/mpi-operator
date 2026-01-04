ARG BASE_LABEL

FROM bash AS downloader

RUN wget https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB -O key.PUB

FROM mpioperator/base:${BASE_LABEL}

COPY --from=downloader key.PUB /tmp/key.PUB

# Install Intel oneAPI keys.
RUN apt update \
    && apt install -y --no-install-recommends gnupg2 ca-certificates apt-transport-https \
    && gpg --dearmor -o /usr/share/keyrings/oneapi-archive-keyring.gpg /tmp/key.PUB \
    && rm /tmp/key.PUB \
    # TODO (tenzen-y): Once Intel OneAPI supports new parsable PGP format for apt, we should remove `trusted=yes` option.
    # REF: https://github.com/kubeflow/mpi-operator/issues/691
    && echo "deb [signed-by=/usr/share/keyrings/oneapi-archive-keyring.gpg trusted=yes] https://apt.repos.intel.com/oneapi all main" | tee /etc/apt/sources.list.d/oneAPI.list \
    && apt update \
    && apt install -y --no-install-recommends \
        dnsutils \
        intel-oneapi-mpi-2021.14 \
    && apt remove -y gnupg2 ca-certificates \
    && apt autoremove -y \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
