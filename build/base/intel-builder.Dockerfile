FROM bash AS downloader

RUN wget https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB -O key.PUB

FROM debian:buster

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
        libstdc++-8-dev binutils procps clang \
        intel-oneapi-compiler-dpcpp-cpp \
        intel-oneapi-mpi-devel \
    && rm -rf /var/lib/apt/lists/*

ENV I_MPI_CC=clang I_MPI_CXX=clang++
