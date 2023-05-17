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
    as used by kubez-ansible project.

author: Caoyingjun
'''

EXAMPLES = '''
- hosts: all
  tasks:
  - name: Install harbor applications by helm_toolbox
    helm_toolbox:
      name: harbor
      namespace: default
      state: present
      repository:
        name: harbor
        url: https://helm.goharbor.io
      chart:
        path: harbor/harbor
        version: 1.9.0
      chart_extra_vars:
        setkey1: setvalue1
        setkey2: setvalue2
      chart_extra_flags:
        - key1
        - key2
        ...

- hosts: all
  tasks:
  - name: Uninstall harbor applications by helm_toolbox
    helm_toolbox:
      name: harbor
      namespace: default
      state: absent
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
        stdout, stderr = stdout.decode(), stderr.decode()
        retcode = proc.poll()
        if retcode != 0:
            output = 'cmd: "%s", code: "%s" stdout: "%s", stderr: "%s"' % (cmd, retcode, stdout, stderr)
            raise Exception(output)
        return stdout.rstrip()

    def present(self):
        # add repo
        self.add_repo()

        # To install the applications by helm3
        # a. check whether the chart installed
        # b. install the chart if not installed or pass
        if self.chart and not self.is_installed:
            cmd = ['helm', 'install', self.name, self.chart.get('path'),
                   '-n', self.namespace, '--kubeconfig', KUBECONFIG]
            if self.chart.get('version'):
                cmd.extend(["--version", self.chart.get('version')])

            if self.params.get('chart_extra_vars'):
                chart_extra_vars = self.params.get('chart_extra_vars')
                if isinstance(chart_extra_vars, dict):
                    chart_extra_cmd = ' '.join('--set {}={}'.format(key, value)  # noqa
                                      for key, value in chart_extra_vars.items() if value)  # noqa

                    cmd.append(chart_extra_cmd)

            if self.params.get('chart_extra_flags'):
                chart_extra_flags = self.params.get('chart_extra_flags')
                if isinstance(chart_extra_flags, list):
                    chart_flags_cmd = ' '.join('--{}'.format(key) for key in chart_extra_flags if key)
                    cmd.append(chart_flags_cmd)

            self.run_cmd(' '.join(cmd))
            self.changed = True

    def absent(self):
        self.remove_repo()

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
            ' '.join(['helm', 'list', '-n', self.namespace, '--kubeconfig', KUBECONFIG])).split('\n')
        for chart in charts:
            if self.name in chart and self.namespace in chart:
                return True
        return False

    # Add repo if it present
    def add_repo(self):
        repo = self.params.get('repository')
        if not repo:
            return

        repo_name = repo.get('name')
        repo_url = repo.get('url')
        if not repo_name or not repo_url:
            raise Exception('name or url not provided when add repository')
        # add repo
        self.run_cmd(' '.join(['helm', 'repo', 'add', repo_name, repo_url]))
        # update repo
        self.run_cmd(' '.join(['helm', 'repo', 'update', repo_name]))

    def remove_repo(self):
        repo = self.params.get('repository')
        if not repo:
            return

        repo_name = repo.get('name')
        if not repo_name:
            raise Exception('name not provided when remove repository')

        self.run_cmd(' '.join(['helm', 'repo', 'remove', repo_name]))
        self.changed = True


def main():
    specs = dict(
        name=dict(required=True, type='str'),
        namespace=dict(required=False, type='str', default='default'),
        state=dict(type='str', default='present', choices=['present', 'absent']),
        repository=dict(type='dict'),
        chart=dict(required=True, type='dict'),
        chart_extra_vars=dict(type='dict'),
        chart_extra_flags=dict(type='list'),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = Helm3Worker(params)
        getattr(hw, params.get('state'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
