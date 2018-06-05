FROM golang:1.10.2-alpine3.7 AS build

# Install tools required to build the project.
# We need to run `docker build --no-cache .` to update those dependencies.
RUN apk add --no-cache git
RUN go get github.com/golang/dep/cmd/dep

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

FROM alpine:3.7
COPY --from=build /bin/mpi-operator /bin/mpi-operator
ENTRYPOINT ["/bin/mpi-operator"]
CMD ["--help"]
