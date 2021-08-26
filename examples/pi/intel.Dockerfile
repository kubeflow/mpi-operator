FROM bash AS downloader

RUN wget https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB -O key.PUB


FROM debian:buster as base

COPY --from=downloader key.PUB /tmp/key.PUB

# Install Intel oneAPI keys.
RUN apt update \
		&& apt install -y --no-install-recommends gnupg2 ca-certificates \
    && apt-key add /tmp/key.PUB \
		&& rm /tmp/key.PUB \
		&& echo "deb https://apt.repos.intel.com/oneapi all main" | tee /etc/apt/sources.list.d/oneAPI.list \
		&& apt remove -y gnupg2 ca-certificates \
		&& apt autoremove -y \
		&& rm -rf /var/lib/apt/lists/*


FROM base as builder

RUN apt update \
		&& apt install -y --no-install-recommends \
			libstdc++-8-dev binutils \
			intel-oneapi-compiler-dpcpp-cpp \
			intel-oneapi-mpi-devel \
		&& rm -rf /var/lib/apt/lists/*

ENV I_MPI_CC=clang I_MPI_CXX=clang++
COPY pi.cc /src/pi.cc
RUN bash -c "source /opt/intel/oneapi/setvars.sh && mpicxx /src/pi.cc -o /pi"


FROM base

RUN apt update \
		&& apt install -y --no-install-recommends \
			openssh-server \
			openssh-client \
			dnsutils \
			intel-oneapi-mpi \
		&& rm -rf /var/lib/apt/lists/*

# Add priviledge separation directoy to run sshd as root.
RUN mkdir -p /var/run/sshd
# Add capability to run sshd as non-root.
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd

RUN useradd -m mpiuser
WORKDIR /home/mpiuser
COPY intel-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
COPY --chown=mpiuser sshd_config .sshd_config
# Allow OpenSSH to talk to containers without asking for confirmation
# by disabling StrictHostKeyChecking.
# mpi-operator mounts the .ssh folder from a Secret. For that to work, we need
# to disable UserKnownHostsFile to avoid write permissions.
# Disabling StrictModes avoids directory and files read permission checks.
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

COPY --from=builder /pi /home/mpiuser/pi