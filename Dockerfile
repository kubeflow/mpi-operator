FROM golang:1.13.6 AS build
  
ADD . /go/src/github.com/kubeflow/mpi-operator
WORKDIR /go/src/github.com/kubeflow/mpi-operator
RUN make

FROM gcr.io/distroless/base-debian10:latest
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/_output/cmd/bin/mpi-operator* /opt/
ENTRYPOINT ["/opt/mpi-operator.v1alpha2"]
CMD ["--help"]
