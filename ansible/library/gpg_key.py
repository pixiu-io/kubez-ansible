# Copyright 2024 YourName
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
import subprocess

import requests
import os
import traceback
from ansible.module_utils.basic import AnsibleModule

DOCUMENTATION = '''
---
module: gpg_key
short_description: >
  Download a GPG key and convert it to GPG format.
description:
  - This module downloads a GPG key from a specified URL, converts it to GPG format, and saves it to a specified file path.

author: puzhihao
'''

EXAMPLES = '''
- hosts: localhost
  tasks:
  - name: Download and convert GPG key
    gpg_key:
      url: 'https://example.com/path/to/gpg/key'
      output_path: '/etc/apt/keyrings/example-keyring.gpg'
'''


class GPGKey(object):
    def __init__(self, params):
        self.params = params
        self.url = self.params.get('url')
        self.output_path = self.params.get('output_path')
        self.temp_path = '/tmp/temp_gpg_key'
        self.changed = False
        self.result = {}

    def download_file(self):
        response = requests.get(self.url, stream=True)
        if response.status_code == 200:
            with open(self.temp_path, 'wb') as file:
                file.write(response.content)
            return True
        else:
            return False

    def convert_key(self):
        result = subprocess.run(['gpg', '--yes', '--dearmor', '-o', self.output_path, self.temp_path],
                                capture_output=True, text=True)
        if result.returncode == 0:
            return True
        else:
            return False

    def create_dir(self):
        os.makedirs(os.path.dirname(self.output_path), exist_ok=True)

    def process(self):
        if os.path.exists(self.output_path):
            self.result['msg'] = 'GPG key already exists'
            return
        self.create_dir()
        if self.download_file():
            self.changed = True
            if self.convert_key():
                self.result['msg'] = 'GPG key downloaded and converted successfully'
            else:
                self.result['msg'] = 'Failed to convert the GPG key'
            os.remove(self.temp_path)
        else:
            self.result['msg'] = 'Failed to download the GPG key'


def main():
    specs = dict(
        url=dict(required=True, type='str'),
        output_path=dict(required=True, type='str'),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    gpg_manager = None
    try:
        gpg_manager = GPGKey(params)
        gpg_manager.process()
        module.exit_json(changed=gpg_manager.changed, result=gpg_manager.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(gpg_manager, 'result', {}))


if __name__ == '__main__':
    main()
