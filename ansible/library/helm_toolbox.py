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

import os
import subprocess

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

HELM_BIN = '/usr/bin/helm'


class HelmWorker(object):

    def __init__(self, params):
        self.params = params
        self.helm_type = self.params.get('helm_type')
        self.module_vars = self.params.get('module_vars')
        self.result = {} 

        self.changed = False

    def install(self):
        # To install the helm3 command
        if not os.path.exists(HELM_BIN): 
            helm_image = self.module_vars.get('helm_image')
            if not self.is_image_exists(helm_image):
                self.run_cmd(' '.join(['docker pull', helm_image]))
                
            self.run_cmd(' '.join(['docker run -d --name helm_toolbox', helm_image]))
            self.run_cmd('docker cp helm_toolbox:/usr/bin/helm /usr/bin/helm')
            self.run_cmd('docker rm helm_toolbox -f')

      if not os.access(HELM_BIN, os.X_OK):
          os.chmod(HELM_BIN, "644")




    def apply(self):
        # To install the applications by helm3
        pass


    def is_image_exists(self, image):
        return True if self.run_cmd('docker images -q ' + image) else False
        

    def (self, name, image):
        self.run_cmd('docker pull ' + image)
        return self.run_cmd('docker run -d --name ' + name + ' ' + image)

    def remove_container(self, name):
        return docker rm helm_toolbox -f

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

def main():
    specs = dict(
        helm_type=dict(required=True, type='str', 
                       choices=['install', 'apply']),
        module_vars=dict(type='json')
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = HelmWorker(params)
        getattr(hw, params.get('helm_type'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))

if __name__ == '__main__':
    main()
