FROM alpine:3.7 AS build

# Install kubectl.
ENV K8S_VERSION v1.10.3
RUN apk add --no-cache wget
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /bin/kubectl

# Copy all project.
# This layer is rebuilt when ever a file has changed in the project directory.
COPY . /go/src/github.com/kubeflow/mpi-operator/

FROM alpine:3.7
COPY --from=build /bin/kubectl /bin/kubectl
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/cmd/kubectl-delivery/deliver_kubectl.sh .
ENTRYPOINT ["./deliver_kubectl.sh"]
