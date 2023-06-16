ARG BASE_LABEL

FROM mpioperator/intel-builder:${BASE_LABEL} as builder

COPY pi.cc /src/pi.cc
RUN bash -c "source /opt/intel/oneapi/setvars.sh && mpicxx /src/pi.cc -o /pi"

FROM mpioperator/intel:${BASE_LABEL}

COPY --from=builder /pi /home/mpiuser/pi
