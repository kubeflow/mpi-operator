FROM debian:buster

RUN apt update && apt install -y --no-install-recommends \
			build-essential \
			libopenmpi-dev \
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
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config
COPY pi.cc /src/pi.cc
RUN mpic++ /src/pi.cc -o /home/mpiuser/pi