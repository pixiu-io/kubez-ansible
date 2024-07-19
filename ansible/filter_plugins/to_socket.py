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

import os

from kubez_ansible.to_socket import to_socket
from kubez_ansible.get_runtime_type import get_runtime_type

DOCUMENTATION = '''
author: Caoyingjun
'''


def find_custom_repo(ctx, *args, **kwargs):
    dest = kwargs.get('dest')
    repo_dir = kwargs.get('repo_dir')

    parts = dest.split('/')
    # 最后一个字符串是 repo 的名称
    repo_name = parts[len(parts) - 1]
    # 拼接自定义仓库全路径
    custom_repo = os.path.join(repo_dir, repo_name)

    if os.path.exists(custom_repo):
        return custom_repo
    else:
        return ctx


class FilterModule(object):
    '''Kubez-ansible custom jinja2 filters '''

    def filters(self):
        return {
            'to_socket': to_socket,
            'get_runtime_type': get_runtime_type,
            'find_custom_repo': find_custom_repo
        }
