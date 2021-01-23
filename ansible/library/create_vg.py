#!/usr/bin/env python

import subprocess
import traceback

from ansible.module_utils.basic import AnsibleModule

class CreateLvFromVg(object):

    def __init__(self, params):
        self.params = params
        self.disk = self.params.get('disk')
        self.pesize = self.params.get('pesize')
        self.vg = self.params.get('vg')
        self.lv = self.params.get('lv')
        self.xfs = self.params.get('xfs')

        self.changed = False
        self.result = {}

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

    def create(self):
        if not self.is_installed:
            self.pesize = '+' + self.pesize
            cmd = ['vgextend', self.vg, self.disk, '&&', 'lvresize', '-L', self.pesize, self.lv, '&&', 'xfs_growfs', self.xfs]

            self.run_cmd(' '.join(cmd))
            self.changed = True


    @property
    def is_installed(self):
        charts = self.run_cmd(
            ' '.join(['pvscan', '|', 'grep', self.disk]))
        if self.disk in charts:
            return True
        else:
       	    return False

def main():
    specs = dict(
        disk=dict(required=True,type='str'),
        pesize=dict(required=True, type='str'),
        vg=dict(required=True,type='str'),
        lv=dict(required=True,type='str'),
        xfs=dict(required=True,type='str'),
        create=dict(type='str',default='create')
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)
    params = module.params

    hw = None
    try:
        hw = CreateLvFromVg(params)
        getattr(hw, params.get('create'))()
        module.exit_json(changed=hw.changed, result=hw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         **getattr(hw, 'result', {}))


if __name__ == '__main__':
    main()