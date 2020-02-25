FROM golang:1.13.6-alpine3.11 AS build

WORKDIR /go/src/github.com/kubeflow/mpi-operator/
COPY . /go/src/github.com/kubeflow/mpi-operator/
RUN GO111MODULE=off go build -o /bin/kubectl-delivery github.com/kubeflow/mpi-operator/cmd/kubectl-delivery

FROM kubeflow/kubectl:v1.14.10
COPY --from=build /bin/kubectl-delivery /bin/kubectl-delivery
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["cp /bin/kubectl /opt/kube/kubectl; /bin/kubectl-delivery -alsologtostderr"]
