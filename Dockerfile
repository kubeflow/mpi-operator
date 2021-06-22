FROM golang:1.15.13 AS build
ARG version=v1alpha2

ADD . /go/src/github.com/kubeflow/mpi-operator
WORKDIR /go/src/github.com/kubeflow/mpi-operator
RUN make mpi-operator.$version

FROM gcr.io/distroless/base-debian10:latest
ARG version=v1alpha2
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/_output/cmd/bin/mpi-operator.$version /opt/mpi-operator
COPY third_party/library/license.txt /opt/license.txt

ENTRYPOINT ["/opt/mpi-operator"]
CMD ["--help"]
