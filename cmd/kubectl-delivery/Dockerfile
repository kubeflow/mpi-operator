FROM alpine:3.8 AS build

# Install kubectl.
ENV K8S_VERSION v1.13.2
RUN apk add --no-cache wget
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /bin/kubectl

FROM alpine:3.8
COPY --from=build /bin/kubectl /bin/kubectl
COPY deliver_kubectl.sh .
ENTRYPOINT ["./deliver_kubectl.sh"]
