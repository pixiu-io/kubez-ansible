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
module: cri_dockerd
short_description: >
  For cri-dockerd installed if necessary.
description:
  - A module targerting at invoking ansible module in cri_dockerd
    as used by kubez-ansible project.

author: Caoyingjun
'''

EXAMPLES = '''
- name: install baniry cri-dockerd
  cri_dockerd:
    name: cri-dockerd
    image: "pixiuio/cri-dockerd:v0.3.10"
  delegate_to: "{{ groups['kube-master'][0] }}"
'''


class DockerWorker(object):
    def __init__(self, params):
        self.params = params
        self.name = self.params.get('name')
        self.image = self.params.get('image')

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
        if not self.is_installed:
            self.install()
            self.changed = True

    def absent(self):
        pass

    def install(self):
        # pull cri-dockerd image
        self.run_cmd(' '.join(['docker', 'pull', self.image]))

        # start cri-dockerd container
        self.clean()
        self.run_cmd(' '.join(['docker', 'run', '-d', '--name', self.name, self.image]))

        # install cri-dockerd
        self.run_cmd(' '.join(['docker', 'cp', self.name + ':/usr/bin/cri-dockerd', '/usr/bin/']))

        # clean cri-dockerd
        self.clean()

    def clean(self):
        if self.exist:
            self.run_cmd(' '.join(['docker', 'rm', self.name, '-f']))

    @property
    def exist(self):
        images = self.run_cmd(' '.join(['docker', 'ps', '--format', "'{{.Names}}'"]))
        for image in images.split('\n'):
            if image == self.name:
                return True

        return False

    @property
    def is_installed(self):
        proc = subprocess.Popen(' '.join(['type', 'cri-dockerd', '>', '/dev/null', '2>&1']),
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, stderr = proc.communicate()
        stdout, stderr = stdout.decode(), stderr.decode()
        retcode = proc.poll()
        if retcode == 0:
            return True

        return False


def main():
    specs = dict(
        name=dict(required=True, type='str'),
        image=dict(required=True, type='str'),
        state=dict(type='str', default='present', choices=['present', 'absent']),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = DockerWorker(params)
        getattr(hw, params.get('state'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()
