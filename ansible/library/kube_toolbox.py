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

DOCUMENTATION = '''
author: Caoyingjun

'''

import os
import subprocess

import traceback


KUBEADMIN = '/etc/kubernetes/admin.conf'


class KubeWorker(object):

    def __init__(self, params):
        self.params = params
        self.module_name = self.params.get('module_name')
        self.module_args = self.params.get('module_args')
        self.is_ha = self.params.get('is_ha')
        self.changed = False
        # Use this to store arguments to pass to exit_json()
        self.result = {}

    @property
    def _is_kube_cluster_exists(self):
        if not os.path.exists(KUBEADMIN):
            return False

        # Export KUBECONFIG into environ
        os.environ['KUBECONFIG'] = KUBEADMIN

        cmd = 'kubectl cluster-info'
        kube_result = self._run(cmd)
        if 'is running at' in kube_result:
            return True
        return False

    @property
    def is_bootstrap(self):
        if (self.module_name == 'kubeadm'
            and self.module_args.startswith('init')):
            return True
        return False

    @property
    def is_worker_add(self):
        if (self.module_name == 'kubeadm'
            and self.module_args.startswith('join')):
            return True
        return False

    @property
    def is_kubectl(self):
        if self.module_name == 'kubectl':
            return True
        return False

    @property
    def commandlines(self):
        cmd = []
        cmd.append(self.module_name)
        cmd.append(self.module_args)
        if self.is_ha:
            control_cmd = '--control-plane-endpoint 127.0.0.1:8443 --upload-certs'
            cmd.append(control_cmd)

        if self.params.get('module_extra_vars'):
            module_extra_vars = self.params.get('module_extra_vars')
            if isinstance(module_extra_vars, dict):
                if self.is_bootstrap:
                    module_extra_vars = ' '.join('--{}={}'.format(key, value)
                                        for key, value in module_extra_vars.items())
                if self.is_worker_add:
                    extra_cmd = ''
                    for key, value in module_extra_vars.items():
                        if key == 'discovery-token-ca-cert-hash':
                            extra_cmd += ' '.join(['--' + key, 'sha256:' + value])
                        else:
                            extra_cmd += ' '.join(['--' + key, value])
                        extra_cmd += ' '

                    module_extra_vars = extra_cmd[:-1]

                cmd.append(module_extra_vars)

        return ' '.join(cmd)

    def _run(self, cmd):
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, _ = proc.communicate()
        return stdout

    def run(self):
        if self.is_bootstrap:
            if not self._is_kube_cluster_exists:
                bootstrap_result = self._run(self.commandlines)
                self.changed = True
                self.result['bootstrap_result'] = bootstrap_result
        else:
            if self.is_kubectl:
                # Export KUBECONFIG into environ
                os.environ['KUBECONFIG'] = KUBEADMIN
            kube_result = self._run(self.commandlines)

            # For idempotence, when is kubectl apply, the changed is always
            # False.
            if not self.module_args.startswith('apply'):
                self.changed = True
            self.result['kube_result'] = kube_result


def main():
    specs = dict(
        module_name=dict(type='str', required=True),
        module_args=dict(type='str', required=True),
        module_extra_vars=dict(type='json'),
        is_ha=dict(type='bool', default=False)
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    bw = None
    try:
        bw = KubeWorker(params)
        bw.run()
        module.exit_json(changed=bw.changed, result=bw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(bw, 'result', {}))


# import module snippets
from ansible.module_utils.basic import *  # noqa
if __name__ == '__main__':
    main()
