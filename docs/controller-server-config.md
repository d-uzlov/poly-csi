
# Server config

This file describes format of server entries in  controller config.

Each server has a few things in the configuration:
- [Name and stableId](#name-and-stableid)
- [Command exec](#command-exec)
- [Volume metadata storage](#volume-metadata-storage)
- [Commands](#commands)
- [List of external addresses](#list-of-external-addresses)
- [`globalParams`](#globalparams)

## Full example

```yaml
name: rose
stableId: 197bf7b716
execType: ssh
execConfig:
  serverAddress: nas-rose.k8s.lan:22
  username: user
  dialTimeout: 1s
  defaultCommandTimeout: 1s
  insecureEchoCommands: false
  auth:
  - name: k8s-secret
    type: key
    valueFromFile: /poly-csi/ssh-keys/ssh_key_1
metadataType: json-file
metadataConfig:
  directory: /etc/poly-csi/volumes/
addressList:
- name: storage
  stableId: 22fb9b60
  address: nas-rose.storage.lan
- name: k8s
  stableId: 6d7d4d62
  address: nas-rose.k8s.lan
commands:
  nvmet_keeper: [ sudo, /opt/nvmet_keeper, --config-folder=/etc/nvmet_keeper ]
globalParams:
  nqnBase: nqn.2025-01.com.example:rose
  iqnBase: iqn.2025-01.com.example:rose
```

### Name and stableId

Both `name` and `stableId` can be used to identify a server in the list.

Name is only relevant when creating new volumes. You can change names without any consequences for existing volumes.

`stableId` is, like the name suggests, a stable identity that does not change.
When volume is created, controller saves value of `stableId`.
`stableId` is later used to find the server for any operations with the volume.

The person writing config MUST make sure that `stableId` never changes.
If you change `stableId`, it will be as if you deleted the old server and created a new one.
You will not be able to change, delete, or sometimes even mount volumes created on this server.

### Command exec

When configuring storage server, you usually need to execute various external commands.

### Command exec: SSH

Usually commands do local configuration, so you need to run on the storage server.

SSH is expected to be the most commonly used exec transport.

Full example:

```yaml
execType: ssh
execConfig:
  serverAddress: example.com:22
  username: user
  defaultCommandTimeout: 1s
  insecureEchoCommands: false
  auth:
  - name: k8s-secret
    type: key
    valueFromFile: /poly-csi/ssh-keys/ssh_key_1
```

- [required] `serverAddress` defines address for SSH connection.
It is used only for management commands from controller,
it's not used for data connections from nodes.
THe port is optional.
- [required] `username` defines username for SSH connection
- `defaultCommandTimeout` (default `1s`): most commands are expected to be near-instantaneous,
but can sometimes hang due to misconfiguration or technical issues.
Default timeout sets a timeout for all such commands.
If you are experiencing timeout errors because your storage server is not able to handle commands quick enough,
you may want to increase default timeout value.
- `insecureEchoCommands` (default `false`): write all commands to logs.
This is intended to be a debugging utility.
Usually error messages should be descriptive enough to understand what went wrong.
But sometimes they aren't. Command echo will write exact commands that controller attempted to run,
so you can read them and/or run them manually, and find out what's wrong.
Note that when `insecureEchoCommands=true`, all commands are printed as is, including any secret values.
- [required] `auth`: a list of SSH authentication options.
Each auth entry describes one auth option.
SSH client will try them one by one, until one is accepted by server.
This config section does not contains secret values by itself.
- - `name`: name of the entry, used only for logs
- - [required] `type`: auth type, allowed values are: `key`, `password`
- - `valueFromEnv`: read environment variable for actual value
- - `valueFromFile`: read file for actual value

---

### Command exec: local

Local exec can be useful when controller is running directly on the storage server.

Full example:

```yaml
execType: local
execConfig: {}
```

Currently local exec does not have any configuration.

### Volume metadata storage

You can find reasoning why volume metadata storage is required in the [design overview](./design.md#extra-components-volume-metadata).

Note that if metadata is lost or corrupted, controller will be unable to manage it.

Full example:

```yaml
metadataType: json-file
metadataConfig:
  directory: /etc/poly-csi/volumes/
```

Currently the only available metadata storage type is `json-file`.

This metadata type writes metadata to files in a folder specified by `directory` parameter.
It requires a proper exec setup, and the specified folder must be accessible by the exec user.

If needed, you can move contents of `directory` into a different parent folder,
as long as you update `directory` parameter accordingly and don't break internal config structure.

### Commands

`commands` are used to specify how to run certain actions.

Different storage and share options may need different commands.
Some commands have default values, some need to be specified.

The format for each command is `[ command, arg1, arg2, agr3, ... ]`.

For example, for ZFS management there is a `zfs` command.
Usually it is available as just `zfs` binary in PATH, and therefore the default value is `[ sudo, zfs ]`.
But it may be that zfs is not in the PATH, and then you would need to specify full path to binary: `[ sudo, /path/to/zfs ]`.
Or you can create a script that can enhance or alter zfs configuration: `[ sudo, /path/to/zfs-hook.sh ]`.

### List of external addresses

`addressList` specifies now storage server can be accessed.

Each entry has the following format:

```yaml
name: human-friendly-name
stableId: 22fb9b60
address: nas.example.com
```

Address does not do anything by itself, volumes will not use all addresses in the list automatically.

When creating new volume, you specify which address should be used by setting `dispatch.addressNames` parameter.

`name` and `stableId` on the address work the same way they do on the server object:
you use `name` when creating volume, and then you can safely change name without affecting existing volumes.

The actual address is not saved in a volume.
Each time a volume is mounted onto a node, controller looks up current address value in the list.
Therefore, you can change value of `address` field, and even old volumes will use new address.
However, already mounted volumes will not be affected. Address is only updated when volume is attached to a node.

### `globalParams`

`globalParams` is an opaque map of string values.

It can be used by storage and share managers.
Refer to storage or share respective config for available values.
