FROM golang:1.15.13 AS build
ARG version=v2

ADD . /go/src/github.com/kubeflow/mpi-operator
WORKDIR /go/src/github.com/kubeflow/mpi-operator
RUN make mpi-operator.v1 mpi-operator.v2
RUN ln -s mpi-operator.$version _output/cmd/bin/mpi-operator

FROM gcr.io/distroless/base-debian10:latest
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/_output/cmd/bin/* /opt/
COPY third_party/library/license.txt /opt/license.txt

ENTRYPOINT ["/opt/mpi-operator"]
CMD ["--help"]
