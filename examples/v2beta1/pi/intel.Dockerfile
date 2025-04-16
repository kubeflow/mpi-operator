ARG BASE_LABEL

FROM mpioperator/intel-builder:${BASE_LABEL} as builder

# Add Intel repository key and update apt sources
RUN curl -fsSL https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB | gpg --dearmor -o /usr/share/keyrings/oneapi-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/oneapi-archive-keyring.gpg] https://apt.repos.intel.com/oneapi all main" \
    > /etc/apt/sources.list.d/oneapi.list \
    && apt update \
    && apt install -y --no-install-recommends gnupg2 ca-certificates apt-transport-https \
    && apt autoremove -y \
    && rm -rf /var/lib/apt/lists/*

COPY pi.cc /src/pi.cc
RUN bash -c "source /opt/intel/oneapi/setvars.sh && mpicxx /src/pi.cc -o /pi"

FROM mpioperator/intel:${BASE_LABEL}

COPY --from=builder /pi /home/mpiuser/pi
