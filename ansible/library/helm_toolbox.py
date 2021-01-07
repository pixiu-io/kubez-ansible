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

import subprocess
import traceback

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

EXAMPLES = '''
- hosts: all
  tasks:
  - name: Install harbor applications by helm3
    helm_toolbox:
      name: harbor
      namespace: default
      chart: chart
      chart_extra_vars:
        setkey1: setvalue1
        setkey2: setvalue2
        ...

- hosts: all
  tasks:
  - name: Uninstall harbor applications by helm3
    helm_toolbox:
      name: harbor
      namespace: default
      action: uninstall
'''

KUBECONFIG = '/etc/kubernetes/admin.conf'


class Helm3Worker(object):

    def __init__(self, params):
        self.params = params
        self.name = self.params.get('name')
        self.namespace = self.params.get('namespace')
        self.chart = self.params.get('chart')

        self.changed = False
        self.result = {}

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
        # a. check whether the chart installed
        # b. install the chart if not installed or pass
        if not self.is_installed:
            cmd = ['helm', 'install', self.name, self.chart,
                   '-n', self.namespace, '--kubeconfig', KUBECONFIG]
            if self.params.get('chart_extra_vars'):
                chart_extra_vars = self.params.get('chart_extra_vars')
                if isinstance(chart_extra_vars, dict):
                    chart_extra_cmd = ' '.join('--set {}={}'.format(key, value)  # noqa
                                      for key, value in chart_extra_vars.items() if value)  # noqa

                    cmd.append(chart_extra_cmd)

            self.run_cmd(' '.join(cmd))
            self.changed = True

    def uninstall(self):
        # To uninstall the applications by helm3
        # a. check whether the chart installed
        # b. uninstall the chart if installed
        if self.is_installed:
            cmd = ['helm', 'uninstall', self.name,
                   '-n', self.namespace, '--kubeconfig', KUBECONFIG]
            self.run_cmd(' '.join(cmd))
            self.changed = True

    @property
    def is_installed(self):
        charts = self.run_cmd(
            ' '.join(['helm', 'list', '--kubeconfig', KUBECONFIG])).split('\n')
        for chart in charts:
            if self.name in chart and self.namespace in chart:
                return True
        return False


def main():
    specs = dict(
        name=dict(required=True, type='str'),
        namespace=dict(required=False, type='str', default='default'),
        action=dict(type='str', default='install', choices=['install',
                                                            'uninstall']),
        chart=dict(required=True, type='str'),
        chart_extra_vars=dict(type='json')
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = Helm3Worker(params)
        getattr(hw, params.get('action'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
