import unittest
import requests
import os
import json
from random import *

BASE_URL = 'http://localhost:7777/api'
TOKEN = os.environ.get('TOKEN')

class AddJobDefinitionTest(unittest.TestCase):
    def test_add_job_definition(self):
        ### Upload job definitions
        definition_files = [
                '../fixtures/basic-job.yaml',
                '../fixtures/encoding-job.yaml',
                '../fixtures/kube-build.yaml',
                '../fixtures/cron-kube-build.yaml',
                '../fixtures/shell_build.yaml',
                '../fixtures/test_job.yaml',
                '../fixtures/http_job.yaml',
                '../fixtures/docker_build.yaml',
                '../fixtures/fork_job.yaml',
                '../fixtures/bad_job.yaml',
                '../fixtures/node_js.yaml',
                '../fixtures/python_stocks.yaml',
                '../fixtures/maven_build.yaml',
                '../fixtures/gradle_build.yaml',
                '../fixtures/android_app.yaml'
                ]
        for df in definition_files:
            print('Uploading %s' % df)
            with open(df) as f:
                definition = f.read()
                headers = {
                    'Content-Type': 'application/yaml',
                    'Authorization': 'Bearer ' + str(TOKEN)
                }
                resp = requests.post(BASE_URL + '/jobs/definitions', headers = headers, data = definition)
                self.assertTrue(resp.status_code == 200 or resp.status_code == 201, resp.text + ' -- ' + df)

if __name__ == '__main__':
    unittest.TestLoader.sortTestMethodsUsing = None
    unittest.main()
