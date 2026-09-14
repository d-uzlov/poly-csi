
# Share config

This file discusses existing options for network share configuration.

Shares export folders or devices from the storage server, but they do not create them.

For storage provision see storage-config file.

## NFS via exports

Type: `nfs-exports`

Global commands:
- `nfs_reload`: default `[ sudo, exportfs, -r ]`

Available options:
- `nfs.shareOptions`: NFS share options. Default: `rw,no_subtree_check,no_root_squash`

Saves state to `/etc/exports.d/<share_id>.exports`.

Requirements: NFS server.

```bash
# Debian and debian-based distros
sudo apt-get install -y nfs-kernel-server
```

## NFS via ZFS shares

Type: `zfs-sharenfs`

Can be used only with volumes created by ZFS storage.

Available options:
- `nfs.shareOptions`: NFS share options. Default: `rw,no_subtree_check,no_root_squash`

Saves state in ZFS dataset properties.

Requirements: NFS server.

```bash
# Debian and debian-based distros
sudo apt-get install -y nfs-kernel-server
```

## iSCSI via LIO/targetcli

Share type: `targetcli`

Uses iSCSI share type.

Global commands:
- `targetcli`: default `[ sudo, targetcli ]`
- `targetcli_ready`: no default, skip readiness check when empty

Global parameters:
- [required] `iqnBase`: prefix for all IQNs. Share IQN will look like `<iqnBase>:<share_id>`

Available options:
- `lio.transport`: TODO remove?
- `lio.listenPort`: TODO port that nodes will connect to. Default: `3260`
- `lio.authType`: describes how iSCSI share is secured. See below for details. Default: `chap-provision`.

Saves state using `targetcli / saveconfig`.

**Auth**: there are several options for iSCSI auth:
- `none`: share is not protected, anyone can connect to it. Can be convenient for debugging.
- `iqn`: iSCSI initiator (client) must provide registered IQN when connecting. Node IQNs are automatically registered in Controller Publish method.
- `chap-provision`: iSCSI initiator (client) must provide matching secret when connecting. Secrets are automatically generated in Controller Publish method.

All LIO/targetcli requirements are usually configured when installing `targetcli` package:

```bash
# Debian and debian-based distros
sudo apt-get install -y targetcli-fb
```

## NVME via `nvmet_keeper`

Type: `nvmet-keeper`

Uses NVMEoF share type.

Global commands:
- `nvmet_keeper`: no default, required to be set.

Global parameters:
- [required] `nqnBase`: prefix for all NQNs. Share NQN will look like `<nqnBase>:<share_id>`

Available options:
- `nvmet.transportType`: NVME transport to use. Default: `tcp`.
- - `rdma` and fibre-channel are not tested because of lack of hardware.
- [required] `nvmet.nvmetPortName`: pre-configured `nvmet` port.
- [required] `nvmet.listenPort`: port on which `nvmet` port is listening.
- [required] `nvmet.authType`: describes how share is secured. See below for details.

Saves state in `nvmet_keeper` config folder.

**Auth**: there are several options for NVMEoF auth:
- `none`: share is not protected, anyone can connect to it. Can be convenient for debugging.
- `nqn`: client must provide registered NQN when connecting. Node NQNs are automatically registered in Controller Publish method.
- `chap-provision`: client must provide matching secret when connecting. Secrets are automatically generated in Controller Publish method.

Requirements:
- `nvmet` kernel module
- `nvmet_keeper` command
- Preconfigured NVME target port

Set `nvmet_keeper` global command to `[ sudo, /opt/nvmet_keeper, --config-folder=/etc/nvmet_keeper ]`.

```bash

# copy nvmet_keeper files from local filesystem to target machine (you need to compile the binary manually)
# TODO replace with download links
target=
scp ./deploy/nvmet_keeper.service $target:./nvmet_keeper.service
scp ./bin/nvmet_keeper $target:./nvmet_keeper

# below actions should be run on the target server

# nvmet kernel module must be loaded
 sudo tee /etc/modules-load.d/nvmeof.conf << EOF
nvmet
nvmet-tcp
nvmet-rdma
EOF
sudo systemctl restart systemd-modules-load.service
# check that modules are loaded
lsmod | grep -e nvmet

sudo mv ./nvmet_keeper /opt/nvmet_keeper
sudo mv ./nvmet_keeper.service /etc/systemd/system/nvmet_keeper.service

sudo systemctl daemon-reload
sudo systemctl enable nvmet_keeper.service
sudo systemctl restart nvmet_keeper.service
sudo systemctl status nvmet_keeper.service --no-pager

# configure port
sudo /opt/nvmet_keeper port create '{"name":"4420","address_family":"ipv4","listen_addr":"0.0.0.0","listen_port":4420,"type":"tcp"}'

```

## NVME via Linux `nvmet` kernel module

Type: `nvmet-kernel`

Uses NVMEoF share type.

[!] `nvmet-kernel` share is provided for historical reasons. It's recommended that you use `nvmet-keeper` instead.

Global commands:
- `nvmet_save`: no default, required to be set.
`nvmet_save` must be a command that reads current state of `nvmet` kernel module and saves it into persistent storage.
See below for options.
- `nvmet_ready`: no default, skip readiness check when empty

Global parameters:
- [required] `nqnBase`: prefix for all NQNs. Share NQN will look like `<nqnBase>:<share_id>`

Available options:
- `nvmet.transport`: NVME transport to use. Default: `tcp`
- `nvmet.nvmetPort`: pre-configured `nvmet` port. Default: `1`
- `nvmet.listenPort`: port on which `nvmet` port is listening. Default: `4420`

NVME target on generic Linux distro does not have a standard way to save state.
This share manager expects to get a custom command that will capture `nvmet` state and make it persistent.
See below for options.

Requirements:
- `nvmet` kernel module
- A way to save state
- Preconfigured NVME target port

```bash

# nvmet kernel module must be loaded
 sudo tee /etc/modules-load.d/nvmeof.conf << EOF
nvmet
nvmet-tcp
nvmet-rdma
EOF
sudo systemctl restart systemd-modules-load.service
# check that modules are loaded
lsmod | grep -e nvmet

```

---

One option for `nvmet` state management is `nvmetcli`.
This project has been around since `nvmet` was added to Linux,
but it does not seem to be well maintained.

For me it works on Debian 12 but fails to start on Debian 13 due to breaking changes in python3-configshell-fb package.

Set `nvmet_save` to `[ sudo, nvmetcli, save, /etc/nvmet/config.json ]`.

References:
- https://git.infradead.org/users/hch/nvmetcli.git

```bash

# nvmetcli is not published as a package, so you need to manually install dependencies, and then install it from source
sudo apt-get install -y git pip python3-six python3-pyparsing python3-configshell-fb python3-setuptools
git clone --depth 1 git://git.infradead.org/users/hch/nvmetcli.git
( cd nvmetcli && sudo python3 setup.py install --prefix=/usr )

# check that it works
sudo nvmetcli ls

# /etc/nvmet is the default location for nvmetcli config file
sudo mkdir -p /etc/nvmet

# you can add ExecStartPost to use in nvmet_ready
sed -E 's|(ExecStart=.*)|\1\nExecStartPost=/usr/bin/touch /var/run/nvmet-config-loaded|' ./nvmetcli/nvmet.service | sudo tee /etc/systemd/system/nvmet.service

sudo systemctl daemon-reload
sudo systemctl enable --now nvmet.service
sudo systemctl status nvmet.service

# create a tcp port listening on all IPs on port 4420, and save config
echo '/ports create 1' | sudo nvmetcli
echo '/ports/1 set addr adrfam=ipv4 trtype=tcp traddr=0.0.0.0 trsvcid=4421' | sudo nvmetcli
sudo nvmetcli save /etc/nvmet/config.json

```

---

Another option for `nvmet` state management is `nvmetcfg`.
This project is pretty new, compared to `nvmetcli`.
It seems like `nvmetcfg` currently lacks support for [host auth management](https://github.com/vifino/nvmetcfg/issues/5)

Refer to github page for installation instructions.

Set `nvmet_save` to `[ sudo, nvmet, state, save, /etc/nvmetcfg/state.yaml ]`.

References:
- https://github.com/vifino/nvmetcfg

---

Another option for `nvmet` state management is `nvmet_keeper`.
Refer to section [NVME via nvmet_keeper](#nvme-via-nvmet_keeper) for installation.

Set `nvmet_save` to `[ sudo, /path/to/nvmet_keeper, config, save-merge ]`.

---

It's possible to use any other tools that can capture and save existing state of `nvmet` kernel config,
you just need to set `nvmet_save` command accordingly.
