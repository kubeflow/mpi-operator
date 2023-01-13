# coding: utf-8

"""
    mpijob

    Python SDK for MPI-Operator  # noqa: E501

    The version of the OpenAPI document: v2beta1
    Generated by: https://openapi-generator.tech
"""


from __future__ import absolute_import

import unittest
import datetime

import mpijob
from mpijob.models.v2beta1_mpi_job import V2beta1MPIJob  # noqa: E501
from mpijob.rest import ApiException

class TestV2beta1MPIJob(unittest.TestCase):
    """V2beta1MPIJob unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test V2beta1MPIJob
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = mpijob.models.v2beta1_mpi_job.V2beta1MPIJob()  # noqa: E501
        if include_optional :
            return V2beta1MPIJob(
                api_version = '', 
                kind = '', 
                metadata = None, 
                spec = mpijob.models.v2beta1/mpi_job_spec.v2beta1.MPIJobSpec(
                    mpi_implementation = '', 
                    mpi_replica_specs = {
                        'key' : mpijob.models.v1/replica_spec.v1.ReplicaSpec(
                            replicas = 56, 
                            restart_policy = '', 
                            template = None, )
                        }, 
                    run_policy = mpijob.models.v1/run_policy.v1.RunPolicy(
                        active_deadline_seconds = 56, 
                        backoff_limit = 56, 
                        clean_pod_policy = '', 
                        scheduling_policy = mpijob.models.v1/scheduling_policy.v1.SchedulingPolicy(
                            min_available = 56, 
                            min_resources = {
                                'key' : None
                                }, 
                            priority_class = '', 
                            queue = '', 
                            schedule_timeout_seconds = 56, ), 
                        ttl_seconds_after_finished = 56, ), 
                    slots_per_worker = 56, 
                    ssh_auth_mount_path = '', ), 
                status = mpijob.models.v1/job_status.v1.JobStatus(
                    completion_time = None, 
                    conditions = [
                        mpijob.models.v1/job_condition.v1.JobCondition(
                            last_transition_time = None, 
                            last_update_time = None, 
                            message = '', 
                            reason = '', 
                            status = '', 
                            type = '', )
                        ], 
                    last_reconcile_time = None, 
                    replica_statuses = {
                        'key' : mpijob.models.v1/replica_status.v1.ReplicaStatus(
                            active = 56, 
                            failed = 56, 
                            label_selector = None, 
                            selector = '', 
                            succeeded = 56, )
                        }, 
                    start_time = None, )
            )
        else :
            return V2beta1MPIJob(
        )

    def testV2beta1MPIJob(self):
        """Test V2beta1MPIJob"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)

if __name__ == '__main__':
    unittest.main()
