FROM golang:1.22 AS build

# Set mpi-operator version
# Defaults to v2
ARG VERSION=v2
ARG RELEASE_VERSION

ADD . /go/src/github.com/kubeflow/mpi-operator
WORKDIR /go/src/github.com/kubeflow/mpi-operator
RUN make RELEASE_VERSION=${RELEASE_VERSION} mpi-operator.$VERSION
RUN ln -s mpi-operator.${VERSION} _output/cmd/bin/mpi-operator

FROM gcr.io/distroless/base-debian12:latest

ENV CONTROLLER_VERSION=$VERSION
COPY --from=build /go/src/github.com/kubeflow/mpi-operator/_output/cmd/bin/* /opt/
COPY third_party/library/license.txt /opt/license.txt

ENTRYPOINT ["/opt/mpi-operator"]
CMD ["--help"]
