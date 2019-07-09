FROM golang:1.12.7-alpine3.10 AS build

WORKDIR /go/src/github.com/kubeflow/mpi-operator/
COPY . /go/src/github.com/kubeflow/mpi-operator/
RUN go build -o /bin/mpi-operator github.com/kubeflow/mpi-operator/cmd/mpi-operator.v1alpha2

FROM alpine:3.10
COPY --from=build /bin/mpi-operator /bin/mpi-operator
ENTRYPOINT ["/bin/mpi-operator"]
CMD ["--help"]
