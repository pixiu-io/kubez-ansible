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

from kubez_ansible.to_socket import to_socket
from kubez_ansible.get_runtime_type import get_runtime_type

DOCUMENTATION = '''
author: Caoyingjun
'''


class FilterModule(object):
    '''Kubez-ansible custom jinja2 filters '''

    def filters(self):
        return {
            'to_socket': to_socket,
            'get_runtime_type': get_runtime_type
        }
