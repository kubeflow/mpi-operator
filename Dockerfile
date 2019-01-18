FROM golang:1.11.4-alpine3.8 AS build

# Install tools required to build the project.
# We need to run `docker build --no-cache .` to update those dependencies.
RUN apk add --no-cache git
ENV DEP_RELEASE_TAG=v0.5.0
RUN wget -O - https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Gopkg.toml and Gopkg.lock lists project dependencies.
# These layers are only re-built when Gopkg files are updated.
COPY Gopkg.lock Gopkg.toml /go/src/github.com/kubeflow/mpi-operator/
WORKDIR /go/src/github.com/kubeflow/mpi-operator/

# Install library dependencies.
RUN dep ensure -vendor-only

# Copy all project and build it.
# This layer is rebuilt when ever a file has changed in the project directory.
COPY . /go/src/github.com/kubeflow/mpi-operator/
RUN go build -o /bin/mpi-operator github.com/kubeflow/mpi-operator/cmd/mpi-operator

FROM alpine:3.8
COPY --from=build /bin/mpi-operator /bin/mpi-operator
ENTRYPOINT ["/bin/mpi-operator"]
CMD ["--help"]
