FROM mpioperator/intel-builder as builder

COPY pi.cc /src/pi.cc
RUN bash -c "source /opt/intel/oneapi/setvars.sh && mpicxx /src/pi.cc -o /pi"

FROM mpioperator/intel

COPY --from=builder /pi /home/mpiuser/pi