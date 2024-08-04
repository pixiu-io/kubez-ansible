# Copyright 2024 Pixiu
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

    def run_cmd(self, cmd):
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, stderr = proc.communicate()
        stdout, stderr = stdout.decode(), stderr.decode()
        retcode = proc.poll()
        if retcode != 0:
            output = 'cmd: "%s", code: "%s" stdout: "%s", stderr: "%s"' % (cmd, retcode, stdout, stderr)
            raise Exception(output)
        return stdout.rstrip()

    def download_file(self):
        response = requests.get(self.url, stream=True)
        if response.status_code == 200:
            with open(self.temp_path, 'wb') as f:
                f.write(response.content)
        else:
            # TODO: raise errors
            raise Exception("failed to download gpg key")

    def convert_key(self):
        self.run_cmd(' '.join(['gpg', '--yes', '--dearmor', '-o', self.output_path, self.temp_path]))
        self.changed = True

    def ensure_gpg_dir_exists(self):
        os.makedirs(os.path.dirname(self.output_path), exist_ok=True)

    def install_gpg(self):
        self.ensure_gpg_dir_exists()

        # download gpg file
        self.download_file()
        self.convert_key()

    def process(self):
        if os.path.exists(self.output_path):
            return

        self.install_gpg()


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
