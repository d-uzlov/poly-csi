
# Client setup / Node setup

This file discusses how to prepare a Linux server to connect to various network storage.

When using block devices, and storage server can have multiple addresses,
you should also read [multipath configuration](./multipath.md).

## NFS

NFS client is part of the Linux kernel.
You don't need any special setup.

TODO is this true? Do you still need `nfs-common` for NFS kernel modules?

```bash
# sudo apt-get install -y nfs-common
```

## Samba / SMB

```bash
sudo apt-get install -y cifs-utils keyutils
```

## iSCSI via iscsiadm

`iscsiadm` is not designed to be managed from a container.
This means that you need to install `iscsiadm` onto the host,
and then allow access to it when mounting volumes on nodes.

```bash
sudo apt-get -y install open-iscsi
```

The easiest way to access `iscsiadm` from container is to mount the whole host root filesystem into container.
In this project this is done in a separate container.

Discussions:
- [Road to containing iSCSI](https://www.docker.com/blog/road-to-containing-iscsi/) by Docker team
- [access to iscsi](https://github.com/siderolabs/extensions/issues/38#issuecomment-1125403043) in Talos
