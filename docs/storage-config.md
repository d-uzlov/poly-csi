
# Storage config

This file discusses existing options for storage provision, and how to configure them.

For exposing volumes as network shares see share-config file.

## Folder

Type: `folder`

Creates folders for volume content.

Can not enforce volume size limit, or report volume size or usage.

Available options:
- [required] `folder.parentFolder`

## ZFS dataset

Type: `zfs-dataset`

Create ZFS datasets.

Global commands:
- `zfs`: default `[ sudo, zfs ]`

Available options:
- [required] `zfs.parentDataset`
- `zfs.customOptions`

ZFS installation and zpool setup are out of scope for this documentation.

Dataset configured in `zfs.parentDataset` must be created manually.

## ZFS zvol

Type: `zfs-zvol`

Create ZFS zvols.

Global commands:
- `zfs`: default `[ sudo, zfs ]`

Available options:
- [required] `zfs.parentDataset`
- `zfs.customOptions`

ZFS installation and zpool setup are out of scope for this documentation.

Dataset configured in `zfs.parentDataset` must be created manually.
