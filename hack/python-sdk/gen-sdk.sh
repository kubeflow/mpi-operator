#!/usr/bin/env bash

# Copyright 2019 The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SWAGGER_JAR_URL="https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/5.1.0/openapi-generator-cli-5.1.0.jar"
SWAGGER_CODEGEN_JAR="hack/python-sdk/openapi-generator-cli.jar"
SWAGGER_CODEGEN_CONF="hack/python-sdk/swagger_config.json"
SWAGGER_CODEGEN_FILE="pkg/apis/kubeflow/v1/swagger.json"
SDK_OUTPUT_PATH="sdk/python"

if [ -z "${GOPATH:-}" ]; then
    export GOPATH=$(go env GOPATH)
fi

# Backup existing openapi_generated.go
mv pkg/apis/kubeflow/v1/openapi_generated.go pkg/apis/kubeflow/v1/openapi_generated.go.backup

echo "Generating OpenAPI specification ..."
openapi-gen --input-dirs github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1,github.com/kubeflow/common/pkg/apis/common/v1 --output-package github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1 --go-header-file hack/boilerplate/boilerplate.go.txt

echo "Generating swagger file ..."
go run hack/python-sdk/main.go 0.1 > ${SWAGGER_CODEGEN_FILE}

echo "Downloading the swagger-codegen JAR package ..."
if ! [ -f ${SWAGGER_CODEGEN_JAR} ]; then
    wget -O ${SWAGGER_CODEGEN_JAR} ${SWAGGER_JAR_URL}
fi

echo "Generating Python SDK for Kubeflow MPI-Operator ..."
java -jar ${SWAGGER_CODEGEN_JAR} generate -i ${SWAGGER_CODEGEN_FILE} -g python-legacy -o ${SDK_OUTPUT_PATH} -c ${SWAGGER_CODEGEN_CONF}

# Rollback the current openapi_generated.go
mv pkg/apis/kubeflow/v1/openapi_generated.go.backup pkg/apis/kubeflow/v1/openapi_generated.go

echo "Kubeflow MPI-Operator Python SDK is generated successfully to folder ${SDK_OUTPUT_PATH}/."
