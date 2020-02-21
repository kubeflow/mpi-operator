FROM golang:1.13.6 AS build
  
ADD . /go/src/github.com/kubeflow/mpi-operator
WORKDIR /go/src/github.com/kubeflow/mpi-operator
RUN go build -o mpi-operator github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2

FROM gcr.io/distroless/base-debian10:latest
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/mpi-operator /opt/
ENTRYPOINT ["/opt/mpi-operator"]
CMD ["--help"]
