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

import traceback

from ansible.module_utils.basic import AnsibleModule

DOCUMENTATION = '''
---
module: cri_socket

author: Caoyingjun
'''

EXAMPLES = '''
- name: install baniry cri-dockerd
  cri_socket:
    runtime_type: "docker"
    kubernetes_version: 1.24.0
  delegate_to: "{{ groups['kube-master'][0] }}"
'''


class SocketWorker(object):
    def __init__(self, params):
        self.params = params
        self.runtime_type = self.params.get('runtime_type')
        self.kubernetes_version = self.params.get('kubernetes_version')

        self.changed = False
        self.result = {}

    def present(self):
        if self.runtime_type == 'containerd':
            self.result['cri_socket'] = '/run/containerd/containerd.sock'
        elif self.runtime_type == 'docker':
            if self.kubernetes_version >= '1.24.0':
                self.result['cri_socket'] = '/var/run/cri-dockerd.sock'
            else:
                self.result['cri_socket'] = ''


def main():
    specs = dict(
        runtime_type=dict(required=True, type='str'),
        kubernetes_version=dict(type='str', required=True),
        state=dict(type='str', default='present', choices=['present']),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = SocketWorker(params)
        getattr(hw, params.get('state'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
