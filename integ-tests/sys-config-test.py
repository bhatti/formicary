import unittest
import requests
import os
import json
from random import *

BASE_URL = 'http://localhost:7777/api'
TOKEN = os.environ.get('TOKEN')

class JobResourcesTest(unittest.TestCase):
    def test_add_job_resources(self):
        headers = {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + str(TOKEN)
        }
        for x in range(5):
            req = {'kind': 'jobs', 'name': 'name_' + str(x), 'value': 'value_' + str(x)}
            resp = requests.post(BASE_URL + '/configs', headers = headers, json = req)
            self.assertEqual(201, resp.status_code)
            self.assertEqual('jobs', resp.json()['kind'])

if __name__ == '__main__':
    unittest.TestLoader.sortTestMethodsUsing = None
    unittest.main()
