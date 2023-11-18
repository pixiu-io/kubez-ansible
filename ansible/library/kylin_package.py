#!/usr/bin/env python
#
# Copyright 2019 Caoyingjun
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
import traceback

from ansible.module_utils.basic import AnsibleModule

DOCUMENTATION = '''
---
module: kylin_package
short_description: >
  Module for invoking ansible module in kylin_package.

author: Caoyingjun
'''


class KylinPackage(object):
    def __init__(self, params):
        self.params = params
        self.name = self.params.get('name')
        self.state = self.params.get('state')

        self.changed = False
        self.result = {}

    def _run(self, cmd):
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, stderr = proc.communicate()
        retcode = proc.poll()

        if retcode != 0:
            output = 'cmd: "%s", code: "%s" stdout: "%s", stderr: "%s"' % (cmd, retcode, stdout, stderr)
            raise Exception(output)
        return stdout

    def present(self):
        if not self.is_installed:
            cmd = ['yum', '-y', 'install', self.name]
            self._run(' '.join(cmd))
            self.changed = True

    def absent(self):
        if self.is_installed:
            cmd = ['yum', '-y', 'remove', self.name]
            self._run(' '.join(cmd))
            self.changed = True

    @property
    def is_installed(self):
        cmd = ['yum', 'list', 'installed', self.name]
        try:
            self._run(' '.join(cmd))
        except Exception:
            return False
        else:
            return True

def main():
    specs = dict(
        name=dict(required=True, type='str'),
        state=dict(type='str', default='present', choices=['present', 'absent']),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = KylinPackage(params)
        getattr(hw, params.get('state'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
