"""A helper, CLI tool for working with py_func's defining E2E tests.

This CLI provides methods for printing and submitting Argo workflows defined
by py_funcs.
"""
import fire
import logging
from kubernetes import client as k8s_client
import retrying
import yaml

from kubeflow.testing import argo_build_util
from kubeflow.testing import run_e2e_workflow
from kubeflow.testing import util

from create_e2e_workflow import create_workflow
# TODO(jlewi): Can we automatically handle the case where py_func points
# to a builder class? Then create the class and call build.
class E2EToolMain:
  """A helper class to add some convenient entry points."""

  def show(self, py_func, name=None, namespace=None, output=None,
           **kwargs): # pylint: disable=no-self-use
    """Print out the workflow spec.

    Args:
      py_func: Dotted name of the function defining the workflow
    """
    kwargs.update({"name": name, "namespace": namespace, })
    workflow = run_e2e_workflow.py_func_import(py_func, kwargs)

    if output:
      logging.info("Dumping workflow to %s", output)
      with open(output, "w") as hf:
        hf.write(yaml.safe_dump(workflow))
    else:
      print(yaml.safe_dump(workflow))

  def apply(self, py_func="kubeflowMPI.mpi_operator.create_e2e_workflow.create_workflow", name=None, namespace=None, # pylint: disable=no-self-use
            dry_run=False,
            open_in_chrome=False, **kwargs):
    """Create the workflow in the current cluster.

    Args:
      py_func: Dotted name of the function defining the workflow
      name: Name for the workflow
      namespace: Namespace to run.
      dry_run: If true modifies the graph to change the command
        to echo the command and delete any working directory
        rather than actually running it.
        This is a quick way to check that the Argo graph is valid.
        Note: This isn't full proof. We also need to
      open_in_chrome: Whether to shell out to chrome to open the
        Argo UI.
      kwargs: Additional args to pass to the python import function
    """
    kwargs.update({"name": name, "namespace": namespace})
    workflow = run_e2e_workflow.py_func_import(py_func, kwargs)
    #print(workflow)
    # Here we have the issues that it expects the config as well, lets pass it in! 
    # This should be good!!!!
    util.load_kube_config(print_config=False)
    client = k8s_client.ApiClient()
    crd_api = k8s_client.CustomObjectsApi(client)
    # Run this on a cluster!!!

    group, version = workflow['apiVersion'].split('/')

    if dry_run:
      logging.warning("Running in dry-run mode. command and workingDir is "
                      "being changed for all steps")

      for t in workflow["spec"]["templates"]:
        if not "container" in t:
          continue

        c = t["container"]
        working_dir = ""

        # Remove the working directory because working directory
        # might not exist if its created by an earlier step
        if "workingDir" in c:
          working_dir = c["workingDir"]
          del c["workingDir"]

        command = c["command"]
        command = " ".join(command)
        command = "Step will run {0}".format(command)
        if working_dir:
          command = "{0} in {1}".format(command, working_dir)

        command = [
          "echo",
          "\"{0}\"".format(command)
        ]

        # Change the command to an echo.
        c["command"] = command

    crd_api.create_namespaced_custom_object(
      group=group,
      version=version,
      namespace=namespace,
      plural='workflows',
      body=workflow)

    # Wait for a status to be returned and print out out
    @retrying.retry
    def get_wf_status():
      result = crd_api.get_namespaced_custom_object(
        group=group,
        version=version,
        namespace=namespace,
        plural='workflows',
        name=name)

      if not "status" in result:
        raise ValueError("Workflow object not ready yet.")
      return result

    result = get_wf_status()
    logging.info("Created workflow:\n%s", yaml.safe_dump(result))

    # TODO(jlewi): We are asumming the workflow is running in the Kubeflow CI
    # cluster. We should try to infer the correct endpoint by looking for an
    # appropriate ingress.
    ui_url = ("http://testing-argo.kubeflow.org/workflows/kubeflow-test-infra/{0}"
            "?tab=workflow".format(name))
    logging.info("URL for workflow: %s", ui_url)

    if open_in_chrome:
      util.run(["google-chrome", ui_url])

  def step_logs(self, workflow, step, project="kubeflow-ci", # pylint: disable=no-self-use
                open_in_chrome=False):
    """Get a stackdriver link to the logs for a particular step.

    Args:
      workflow: Name of the workflow
      step: The name of the step to get the logs for
    """
    url = argo_build_util.logs_link_for_step(workflow, step, project=project)

    print("URL for workflow {0} step {1} in project {2}".format(
      workflow, step, project))
    print(url)

    if open_in_chrome:
      util.run(["google-chrome", url])

if __name__ == "__main__":
  logging.basicConfig(level=logging.INFO,
                      format=('%(levelname)s|%(asctime)s'
                              '|%(pathname)s|%(lineno)d| %(message)s'),
                      datefmt='%Y-%m-%dT%H:%M:%S',
                      )
  logging.getLogger().setLevel(logging.INFO)
  fire.Fire(E2EToolMain)
