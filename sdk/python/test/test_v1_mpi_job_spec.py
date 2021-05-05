# coding: utf-8

"""
    mpijob

    Python SDK for MPI-Operator  # noqa: E501

    The version of the OpenAPI document: v0.1
    Generated by: https://openapi-generator.tech
"""


from __future__ import absolute_import

import unittest
import datetime

import mpijob
from mpijob.models.v1_mpi_job_spec import V1MPIJobSpec  # noqa: E501
from mpijob.rest import ApiException

class TestV1MPIJobSpec(unittest.TestCase):
    """V1MPIJobSpec unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test V1MPIJobSpec
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = mpijob.models.v1_mpi_job_spec.V1MPIJobSpec()  # noqa: E501
        if include_optional :
            return V1MPIJobSpec(
                clean_pod_policy = '', 
                main_container = '', 
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
                        queue = '', ), 
                    ttl_seconds_after_finished = 56, ), 
                slots_per_worker = 56
            )
        else :
            return V1MPIJobSpec(
                mpi_replica_specs = {
                    'key' : mpijob.models.v1/replica_spec.v1.ReplicaSpec(
                        replicas = 56, 
                        restart_policy = '', 
                        template = None, )
                    },
        )

    def testV1MPIJobSpec(self):
        """Test V1MPIJobSpec"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)

if __name__ == '__main__':
    unittest.main()
