FROM mpioperator/openmpi-builder as builder

COPY pi.cc /src/pi.cc
RUN mpic++ /src/pi.cc -o /pi


FROM mpioperator/openmpi

COPY --from=builder /pi /home/mpiuser/pi