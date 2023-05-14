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

DOCUMENTATION = '''
author: Caoyingjun
'''


class ServiceCheck(object):

    def __init__(self, params):
        self.params = params
        self.service_name = self.params.get('service_name')
        self.service_type = self.params.get('service_type')
        self.changed = False

    def _run(self, cmd):
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE)
        stdout, stderr = proc.communicate()
        stdout, stderr = stdout.decode(), stderr.decode()

        retcode = proc.poll()
        # The 3 means command run success but the status is not active
        if retcode not in [0, 3]:
            output = 'stdout: "%s", stderr: "%s"' % (stdout, stderr)
            raise subprocess.CalledProcessError(retcode, cmd, output)
        return stdout, retcode

    def run(self):
        if self.service_type == 'systemd':
            cmd = ['systemctl', 'is-active']
            cmd.append(self.service_name)
        else:
            cmd = ['docker', 'ps', '--format', '"{{.Status}}"', '-f']
            filter_item = '='.join(['name', self.service_name])
            cmd.append(filter_item)

        stdout, retcode = self._run(cmd)
        # When service status is not active, that's means the service
        # should be started, set changed to True to notify started action.
        if self.service_type == 'systemd':
            if retcode == 3 or not stdout.startswith('active'):
                self.changed = True
        else:
            if not stdout.startswith('"Up'):
                self.changed = True


def main():
    specs = dict(
        service_name=dict(type='str', required=True),
        service_type=dict(type='str',
                          choices=['systemd', 'container'],
                          default='systemd')
    )

    module = AnsibleModule(argument_spec=specs, bypass_checks=True) # noqa
    params = module.params

    sc = None
    try:
        sc = ServiceCheck(params)
        sc.run()
        module.exit_json(changed=sc.changed)
    except Exception as emsg:
        module.fail_json(changed=True, msg=emsg, faild=True)


# import module snippets
from ansible.module_utils.basic import *  # noqa
if __name__ == '__main__':
    main()
