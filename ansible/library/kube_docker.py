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

import traceback
import subprocess

import docker

master_images = ['kube-apiserver',
                 'kube-controller-manager',
                 'kube-scheduler',
                 'etcd',
                 'coredns',
                 'kube-proxy',
                 'pause']
worker_images = ['coredns', 'kube-proxy', 'pause']


def get_docker_client():
    return docker.APIClient()


class DockerWorker(object):

    def __init__(self, module):
        self.module = module
        self.params = self.module.params
        self.changed = False
        self.result = {}
        self.kube_image = self.params.get('kube_image')
        self.kube_repo = self.params.get('kube_repo')
        self.kube_version = self.params.get('kube_version')
        self.dc = get_docker_client()

    @property
    def images_exists(self):
        images = self.dc.images()
        return [image['RepoTags'][0]
                for image in images if image['RepoTags']]

    def pull_image(self):
        if self.kube_image not in self.images_exists:
            image_name = self.kube_image.split('/')[1]

            # NOTE(caoyingjun) Pull the images from ali repo.
            ali_image = '/'.join([self.kube_repo, image_name])
            self.dc.pull(ali_image)
            self.dc.tag(ali_image, self.kube_image, force=True)
            if self.params.get('cleanup'):
                self.dc.remove_image(ali_image, force=True)
            self.changed = True

    def get_image(self):
        cmd = ('kubeadm config images list '
               '--kubernetes-version {kube_version}'.format(
               kube_version=self.kube_version))
        proc = subprocess.Popen(cmd,
                                stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE,
                                shell=True)
        stdout, _ = proc.communicate()
        if proc.returncode == 0:
            images_list = []

            for image in stdout.split():
                image_repo, image_tag = image.split(':')
                image_name = image_repo.split('/')[-1]
                if image_name in master_images:
                    images_list.append({'image_repo': image_repo,
                                        'image_tag': image_tag,
                                        'group': 'kube-master'})
                if image_name in worker_images:
                    images_list.append({'image_repo': image_repo,
                                        'image_tag': image_tag,
                                        'group': 'kube-worker'})

            self.result['images_list'] = images_list
        else:
            raise Exception('Get kube images failed')

def main():

    specs = dict(
        kube_image=dict(type='str', default=''),
        kube_repo=dict(type='str', required=True),
        kube_version=dict(type='str', required=True),
        image_action=dict(type='str', default='pull'),
        cleanup=dict(type='bool', default=True),
    )
    module = AnsibleModule(argument_spec=specs, bypass_checks=True)  # noqa

    dw = None
    try:
        dw = DockerWorker(module)
        getattr(dw, '_'.join([module.params.get('image_action'), 'image']))()
        module.exit_json(changed=dw.changed, result=dw.result)
    except Exception:
        module.fail_json(changed=True, msg=repr(traceback.format_exc()),
                         failed=True)


# import module snippets
from ansible.module_utils.basic import *  # noqa
if __name__ == '__main__':
    main()
