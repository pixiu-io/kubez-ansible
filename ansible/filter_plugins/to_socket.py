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


RUNTIME_MAP = {
    'docker-master': 'docker',
    'containerd-master': 'containerd',
    'docker-node': 'docker',
    'containerd-node': 'containerd'
}

SOCKET_MAP = {
    'docker': '',
    'containerd': '--cri-socket /run/containerd/containerd.sock'
}


class FilterModule(object):
    '''Kubez-ansible custom jinja2 filters '''

    def filters(self):
        return {
            'to_socket': self.to_socket,
            'get_runtime_type': self.get_runtime_type
        }

    def get_runtime_type(self, *args, **kwargs):
        kube_group = kwargs.get('kube_group')
        return RUNTIME_MAP[kube_group]

    def to_socket(self, *args, **kwargs):
        kube_group = kwargs.get('kube_group')
        if kube_group.startswith('dokcer'):
            runtime_type = 'docker'
        elif kube_group.startswith('containerd'):
            runtime_type = 'containerd'
        else:
            runtime_type = ''

        if not runtime_type:
            return self

        return ' '.join([self, SOCKET_MAP[runtime_type]])
