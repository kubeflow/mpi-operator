FROM debian:buster as builder

RUN apt update && apt install -y --no-install-recommends \
			g++ \
			libopenmpi-dev \
			&& rm -rf /var/lib/apt/lists/*

COPY pi.cc /src/pi.cc
RUN mpic++ /src/pi.cc -o /pi


FROM debian:buster

RUN apt update && apt install -y --no-install-recommends \
			openmpi-bin \
			openssh-server \
			openssh-client \
			&& rm -rf /var/lib/apt/lists/*
# Add priviledge separation directoy to run sshd as root.
RUN mkdir -p /var/run/sshd
# Add capability to run sshd as non-root.
RUN setcap CAP_NET_BIND_SERVICE=+eip /usr/sbin/sshd

RUN useradd -m mpiuser
WORKDIR /home/mpiuser
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