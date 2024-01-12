Etcd
=========

A role to perform backup of etcd data.

Requirements
------------

- Ansible 2.9 or later

Role Variables
--------------

The following variables can be set for this role:

- `only_once`: Boolean value indicating whether to perform  backup once or not. Defaults to `true`.
- `keep_num`: The number of backups to keep. Defaults to `3`.
- `schedule`: Cron exp to do backup  regular.
- `backup_image`: Use which image to do this work.
- `etcd_endpoints`: The endpoint URL for the Etcd server.
- `etcd_pki_dir`: Which dir hold the etcd pki.
- `etcd_cacert`: The access cacert for the Etcd server.
- `etcd_cert`: The access cert for the Etcd server.
- `etcd_key`: The access key for the Etcd server.
- `backup2minio`: Boolean value indicating whether to perform backup to MinIO object storage or not. Defaults to `false`.
- `s3_bucket`: The name of the bucket in the MinIO server where backups will be stored.
- `backup_dir`: The local directory path where etcd backups will be stored. Defaults to `/var/backups`.

- `tz`: Time zone- and Asia/Shanghai is default.

- `s3["endpoint"]`: The endpoint URL for the MinIO server.

- `s3["secretAccessKey"]`: The secret key for the MinIO server.

- `s3["accessKeyId"]`: The access key for the MinIO server.

Dependencies
------------

None.

Example Playbook
----------------

Including an example of how to use your role (for instance, with variables passed in as parameters) is always nice for users too:

```sh
- hosts: etcd_servers
  roles:
    - role: etcd
      vars:
        only_once: "yes"
        keep_num: "\"3\""
        schedule: "0 3 * * *"
        backup_image: tyvek2zhang/etcd-backup:v3.5.11
        etcd_endpoints: https://localhost:2379
        etcd_pki_dir: /etc/kubernetes/pki/etcd
        etcd_cacert: "{{ etcd_pki_dir }}/ca.crt"
        etcd_cert: "{{ etcd_pki_dir }}/server.crt"
        etcd_key: "{{ etcd_pki_dir }}/server.key"
        backup2minio: "\"0\""
        s3_bucket: etcd-backups
        backup_dir: /var/backups
        tz: Asia/Shanghai
        s3["endpoint"]: http://172.17.16.13:9000
        s3["secretAccessKey"]: minioadmin
        s3["accessKeyId"]: minioadmin
```

License
-------

 [Apache-2.0](http://www.apache.org/licenses)

Author Information
------------------

For any issues or suggestions, please contact the author at [tyvekzhang@gmail.com].
