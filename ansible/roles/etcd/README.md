Etcd
=========

A role to perform backup of etcd data.

Requirements
------------

- Ansible 2.9 or later

Role Variables
--------------

The following variables can be set for this role:

- `etcd_backup_local`: Boolean value indicating whether to perform local backup or not. Defaults to `true`.

- `etcd_ver`: Etcd version in hand.

- `etcd_backup_local_path`: The local directory path where etcd backups will be stored. Defaults to `/var/backups/etcd`.

- `etcd_backup_keep`: The number of backups to keep. Defaults to `3`.

- `etcd_backup_endpoints`: The endpoint URL for the Etcd server.

- `etcd_backup_cacert`: The access cacert for the Etcd server.

- `etcd_backup_cert`: The access cert for the Etcd server.

- `etcd_backup_key`: The access key for the Etcd server.

- `etcd_backup_minio`: Boolean value indicating whether to perform backup to MinIO object storage or not. Defaults to `false`.

- `etcd_backup_minio_endpoint`: The endpoint URL for the MinIO server.

- `etcd_backup_minio_access_key`: The access key for the MinIO server.

- `etcd_backup_minio_secret_key`: The secret key for the MinIO server.

- `etcd_backup_minio_bucket`: The name of the bucket in the MinIO server where backups will be stored.


Dependencies
------------

None.

Example Playbook
----------------

Including an example of how to use your role (for instance, with variables passed in as parameters) is always nice for users too:

```yaml
- hosts: etcd_servers
  roles:
    - role: etcd
      vars:
        etcd_backup_local: true
        etcd_ver: v3.5.11
        etcd_backup_local_path: "/var/backups/etcd"
        etcd_backup_keep: 3
        etcd_backup_endpoints: https://127.0.0.1:2379
        etcd_backup_cacert: /etc/kubernetes/pki/etcd/ca.crt
        etcd_backup_cert: /etc/kubernetes/pki/etcd/server.crt
        etcd_backup_key: /etc/kubernetes/pki/etcd/server.key
        etcd_backup_minio: false  # When set to true, you should fill in the information below with your own
        etcd_backup_minio_endpoint: "minio.example.com:9000"
        etcd_backup_minio_access_key: "minio-access-key"
        etcd_backup_minio_secret_key: "minio-secret-key"
        etcd_backup_minio_bucket: "your-buckets"
```

License
-------

 [Apache-2.0](http://www.apache.org/licenses)

Author Information
------------------

For any issues or suggestions, please contact the author at [tyvekzhang@gmail.com].
