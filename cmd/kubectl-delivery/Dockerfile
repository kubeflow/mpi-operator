FROM golang:1.13.6 AS build

WORKDIR /go/src/github.com/kubeflow/mpi-operator/
COPY . /go/src/github.com/kubeflow/mpi-operator/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w" -o /bin/kubectl-delivery github.com/kubeflow/mpi-operator/cmd/kubectl-delivery
# Install kubectl
ENV K8S_VERSION v1.15.0
RUN apt-get install wget
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /bin/kubectl

FROM alpine:3.11.6
COPY --from=build /bin/kubectl-delivery /bin/kubectl-delivery
COPY --from=build /bin/kubectl /bin/kubectl
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["cp /bin/kubectl /opt/kube/kubectl; /bin/kubectl-delivery -alsologtostderr"]
