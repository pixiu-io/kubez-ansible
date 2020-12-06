#!/usr/bin/env python
#
# Copyright 2020 Caoyingjun
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

import os
import subprocess

from ansible.module_utils.basic import AnsibleModule

DOCUMENTATION = '''
---
module: helm_toolbox
short_description: >
  Module for invoking ansible module in helm_toolbox.
description:
  - A module targerting at invoking ansible module in helm_toolbox
    as used by Kubez-ansible project.

author: Caoyingjun
'''


class Helm3Worker(object):

    def __init__(self, params):
        self.params = params
        self.name = self.params.get('name')
        self.namespace = self.params.get('namespace')
        self.chart = self.params.get('chart')

        self.changed = False

    def run_cmd(self, cmd):
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, stderr = proc.communicate()
        retcode = proc.poll()
        if retcode != 0:
            output = 'stdout: "%s", stderr: "%s"' % (stdout, stderr)
            raise subprocess.CalledProcessError(retcode, cmd, output)
        return stdout.rstrip()

    def install(self):
        # To install the applications by helm3
        # 1. check whether the chart installed
        # 2. install the chart if not installed
        if not self.is_installed:
            self.run_cmd(' '.join(['helm', 'install',
                                   self.name, self.chart,
                                   '-n', self.namespace]))
            self.changed = True

    @property
    def is_installed(self):
        charts = self.run_cmd('helm list').split('\n')
        for chart in charts:
            if self.name in chart and self.namespace in chart:
                return True
        return False


def main():
    specs = dict(
        name=dict(required=True, type='str'),
        namespace=dict(required=False, type='str', default='default'),
        chart=dict(required=True, type='str')
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = Helm3Worker(params)
        getattr(hw, 'install')()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
