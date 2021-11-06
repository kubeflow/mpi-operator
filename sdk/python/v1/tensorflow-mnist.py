#!/usr/bin/env python
# coding: utf-8

# In[1]:


from kubernetes.client import V1PodTemplateSpec
from kubernetes.client import V1ObjectMeta
from kubernetes.client import V1PodSpec
from kubernetes.client import V1Container
from kubernetes.client import V1ResourceRequirements


# In[2]:


from mpijob import V1ReplicaSpec
from mpijob import V1MPIJob
from mpijob import V1MPIJobSpec


# In[3]:


from kubernetes import client, config


# In[4]:


default_image = "docker.io/kubeflow/mpi-horovod-mnist"
launcher_command = "mpirun"
launcher_args = [ 
    "-np", "2", 
    "--allow-run-as-root",
    "-bind-to", "none",
    "-map-by", "slot",
    "-x", "LD_LIBRARY_PATH",
    "-x", "PATH",
    "-mca", "pml", "ob1",
    "-mca", "btl", "^openib",
    "python", "/examples/tensorflow_mnist.py"
]

launcher_container = V1Container(
    name="mpi-launcher",
    image=default_image,
    command=launcher_command,
    args=launcher_args,
    resources=V1ResourceRequirements(
        limits={"cpu":"1", "memory":"2Gi"}
    )
)

worker_container = V1Container(
    name="mpi-worker",
    image=default_image,
    command=launcher_command,
    args=launcher_args,
    resources=V1ResourceRequirements(
        limits={"cpu":"2", "memory":"4Gi"}
    )
)


# In[5]:


launcher_spec = V1ReplicaSpec(
    replicas=1,
    template=V1PodTemplateSpec(
        spec=V1PodSpec(
            containers=[launcher_container]
        )
    )
)

worker_spec = V1ReplicaSpec(
    replicas=2,
    template=V1PodTemplateSpec(
        spec=V1PodSpec(
            containers=[worker_container]
        )
    )
)


# In[6]:


job = V1MPIJob(
    kind="MPIJob",
    api_version="kubeflow.org/v1",
    metadata=V1ObjectMeta(
        name="tensorflow-mnist",
    ),
    spec=V1MPIJobSpec(
        slots_per_worker=1,
        mpi_replica_specs={
            "Launcher":launcher_spec,
            "Worker":worker_spec
        }
    )
)


# In[7]:


config.load_kube_config()


# In[8]:


crd_api = client.CustomObjectsApi()


# In[9]:


crd_api.create_namespaced_custom_object(
    group="kubeflow.org",
    version="v1",
    namespace="default",
    plural="mpijobs",
    body=job
)
