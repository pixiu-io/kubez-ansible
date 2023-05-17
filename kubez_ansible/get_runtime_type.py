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

DOCUMENTATION = '''
author: Caoyingjun
'''

RUNTIME_MAP = {
    'docker-master': 'docker',
    'containerd-master': 'containerd',
    'docker-node': 'docker',
    'containerd-node': 'containerd'
}


def get_runtime_type(ctx, *args, **kwargs):
    kube_group = kwargs.get('kube_group')
    return RUNTIME_MAP[kube_group]
