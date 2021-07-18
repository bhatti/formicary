import unittest
import requests
import os
import json
from random import *

BASE_URL = 'http://localhost:7777/api'
TOKEN = os.environ.get('TOKEN')

class SubmitJobRequestTest(unittest.TestCase):
    def test_jobs(self):
        ### Upload job definitions
        definition_files = ['../fixtures/basic-job.yaml',
                '../fixtures/encoding-job.yaml',
                '../fixtures/kube-build.yaml',
                '../fixtures/cron-kube-build.yaml',
                '../fixtures/shell_build.yaml',
                '../fixtures/test_job.yaml',
                '../fixtures/fork_job.yaml',
                '../fixtures/http_job.yaml',
                '../fixtures/bad_job.yaml',
                '../fixtures/android_app.yaml',
                '../fixtures/node_js.yaml',
                '../fixtures/python_stocks.yaml',
                '../fixtures/maven_build.yaml',
                '../fixtures/gradle_build.yaml',
                '../fixtures/docker_build.yaml']
        for df in definition_files:
            with open(df) as f:
                definition = f.read()
                headers = {
                    'Content-Type': 'application/yaml',
                    'Authorization': 'Bearer ' + str(TOKEN)
                }
                resp = requests.post(BASE_URL + '/jobs/definitions', headers = headers, data = definition)
                self.assertTrue(resp.status_code == 200 or resp.status_code == 201, resp.text + ' -- ' + df)
        ###
        ### Submit job requets
        headers = {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + str(TOKEN)
        }
        for df in ['shell-build', 'kube-build', 'basic-job', 'encoding-job', 'test-job', 'docker_build', 'fork-job', 'android-app', 'node-js', 'io.formicary.python-stocks', 'maven-build', 'gradle-build']:
        #for df in ['kube-build']:
        #for df in ['docker_build']:
        #for df in ['python-stocks']:
            for x in range(1):
                req = {'job_type': df, 'platform': 'LINUX', 'job_priority': randint(1, 100),
                        'params': {'Token': 'token-' + str(x), 'IsWindowsPlatform': False, 'Platform': 'LINUX', 'OSVersion': '20.04.1', 'Language': 'Kotlin', 'SleepSecs': 1}}
                print('Submitting %s' % df)
                resp = requests.post(BASE_URL + '/jobs/requests?key1=1&key2=true', headers = headers, json = req)
                self.assertEqual(201, resp.status_code, resp.text + ' -- ' + df)
                self.assertEqual('PENDING', resp.json()['job_state'])
                self.assertEqual('LINUX', resp.json()['platform'])
                self.assertEqual(df, resp.json()['job_type'])

if __name__ == '__main__':
    unittest.TestLoader.sortTestMethodsUsing = None
    unittest.main()
