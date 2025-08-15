FROM bash AS downloader

RUN wget https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB -O key.PUB

FROM debian:trixie

COPY --from=downloader key.PUB /tmp/key.PUB

# Install Intel oneAPI keys.
RUN apt update \
    && apt install -y --no-install-recommends gnupg2 ca-certificates apt-transport-https \
    && gpg --dearmor -o /usr/share/keyrings/oneapi-archive-keyring.gpg /tmp/key.PUB \
    && rm /tmp/key.PUB \
    && echo "deb [signed-by=/usr/share/keyrings/oneapi-archive-keyring.gpg] https://apt.repos.intel.com/oneapi all main" | tee /etc/apt/sources.list.d/oneAPI.list \
    && apt update \
    && apt install -y --no-install-recommends \
        libstdc++-12-dev binutils procps clang \
        intel-oneapi-compiler-dpcpp-cpp \
        intel-oneapi-mpi-devel-2021.13 \
    && apt remove -y gnupg2 ca-certificates apt-transport-https \
    && apt autoremove -y \
    && rm -rf /var/lib/apt/lists/*

ENV I_MPI_CC=clang I_MPI_CXX=clang++
