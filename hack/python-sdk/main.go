/*
Copyright 2019 kubeflow.org.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/spec"
	"k8s.io/klog"
	"k8s.io/kube-openapi/pkg/common"

	mpijobv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	mpijobv2 "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/v2beta1"
)

// Generate OpenAPI spec definitions for MPIJob Resource
func main() {
	if len(os.Args) <= 1 {
		klog.Fatal("Supply the MPIJob version")
	}

	version := os.Args[1]
	if version != "v1" && version != "v2beta1" {
		fmt.Println("Only `v1` or `v2beta1` for MPIJob is supported now")
	}

	filter := func(name string) spec.Ref {
		return spec.MustCreateRef(
			"#/definitions/" + common.EscapeJsonPointer(swaggify(name)))
	}
	var oAPIDefs map[string]common.OpenAPIDefinition
	if version == "v1" {
		oAPIDefs = mpijobv1.GetOpenAPIDefinitions(filter)
	} else if version == "v2beta1" {
		oAPIDefs = mpijobv2.GetOpenAPIDefinitions(filter)
	}
	defs := spec.Definitions{}
	for defName, val := range oAPIDefs {
		defs[swaggify(defName)] = val.Schema
	}
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger:     "2.0",
			Definitions: defs,
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "mpijob",
					Description: "Python SDK for MPI-Operator",
					Version:     version,
				},
			},
		},
	}
	jsonBytes, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(string(jsonBytes))
}

func swaggify(name string) string {
	name = strings.Replace(name, "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/", "", -1)
	name = strings.Replace(name, "github.com/kubeflow/mpi-operator/v2/pkg/apis/kubeflow/", "", -1)
	name = strings.Replace(name, "github.com/kubeflow/common/pkg/apis/common/", "", -1)
	name = strings.Replace(name, "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/", "", -1)
	name = strings.Replace(name, "k8s.io/api/core/", "", -1)
	name = strings.Replace(name, "k8s.io/apimachinery/pkg/apis/meta/", "", -1)
	name = strings.Replace(name, "k8s.io/apimachinery/pkg/runtime/", "", -1)
	name = strings.Replace(name, "k8s.io/apimachinery/pkg/api/", "", -1)
	name = strings.Replace(name, "k8s.io/kubernetes/pkg/controller/", "", -1)
	name = strings.Replace(name, "k8s.io/client-go/listers/core/", "", -1)
	name = strings.Replace(name, "k8s.io/client-go/util/workqueue", "", -1)
	name = strings.Replace(name, "/", ".", -1)
	return name
}
