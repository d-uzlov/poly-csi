
# Server setup

This file discusses how to prepare a Linux server to serve as storage for poly-csi.

You need a few things for a working storage server:
- Access to server management (SSH with superuser rights)
- Storage (existing ZFS pool, existing BTRFS volume, preconfigured LVM, folder in filesystem, etc.)
- Ability to expose storage to network as shares (NFS server, iSCSI target, NVMEoF kernel module, etc.)

## SSH

There are no special requirements on SSH access.

Supported authentication methods:
- Password
- Private key

poly-csi needs to run a bunch of commands as root.
The easiest way to do this is to allow `sudo` for all commands:

```bash
echo "$USER ALL=(ALL:ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/$USER
```

## Storage

See [storage-config.md](./storage-config.md).

## Shares

See [share-config.md](./share-config.md).
