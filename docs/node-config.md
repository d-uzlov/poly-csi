
# Node config

This file discusses format of the node-deployment config.

Usually you don't need to customize node config.
But in case you do, here's the spec.

## Full example

```yaml
nodeInfoDb:
  address: http://poly-csi-node-registry:8080
  interval: 15s

encrypt:
  nodeKey:
    path: /poly-csi/node/node_key.pem
  controllerPublicKey:
    path: /poly-csi/controller/controller_key.pub.pem

multipath:
  id: multipath
  type: plugin
  subtype: http.json.dev
  config:
    socket: /run/io.meoe.poly-csi/plugins/multipath-container/multipath.sock

mounters:
- id: nfs
  type: builtin
  subtype: nfs
- id: nvme
  type: builtin
  subtype: nvme
- id: iscsi
  type: plugin
  subtype: http.json.dev
  config:
    socket: /run/io.meoe.poly-csi/plugins/iscsi-container/iscsi.sock
```

## Node info

`nodeInfoDb` describes connection to node info database.

Example:

```yaml
address: http://poly-csi-node-registry:8080
interval: 15s
```

- `address` is a fully-specified address of the database, including schema and port.

- `interval` determines how ofter node info is updated.
Node info is instantly updated when deployment starts, and after config reload, regardless of the interval setting.

Periodic updates are needed because node info DB is expected to be ephemeral, it can lost its data on restart.

## Encryption

`encrypt` describes how node can encrypt and decrypt secret data, passing between node and controller components.

Full example:

```yaml
nodeKey:
  path: /poly-csi/node/node_key.pem
controllerPublicKey:
  path: /poly-csi/controller/controller_key.pub.pem
```

`nodeKey.path` specifies path to node private key file.
If file at specified path does not exist, the app will generate a new key and save it to disk.
If file exists, it must be an RSA private key in PKCS#8 form.
Node will extract public key from it, and send it to node info DB, to be able to decode secrets from controller.

`controllerPublicKey.path` specifies path to controller public key file.
The key format must be RSA.
Node will use it to encrypt secret node data that controller needs access to.

## Multipath

`multipath` describes what node will do when volume is connected as several separate devices.

For example, this happens when storage server has more than one address, and you use iSCSI for connection.

Full example:

```yaml
id: multipath
type: plugin
subtype: http.json.dev
config:
  socket: /run/io.meoe.poly-csi/plugins/multipath-container/multipath.sock
```

The format is the same as `mounters`, except it uses different types:

- `builtin/none`: disable multipath
- `builtin/local`: use local `multipath` binary
- `plugin/http.json.dev`

Multipath docs:
- https://manpages.debian.org/testing/multipath-tools/multipath.conf.5.en.html
- https://man.archlinux.org/man/multipath.conf.5.en

## Mounters

`mounters` is a list of all possible ways a node can connect to storage.

Each entry in the list has the following structure:

```yaml
id: example
type: builtin
subtype: actual_mounter_type
config:
  qwe: asd
```

`id` is the main identifier of mounter.
It can be an arbitrary value, consisting of `0-9` digits, `a-z` lower case latin letters, or `-` dash symbols.

`type + subtype` determine full mounter type.
The following types are available:
- `builtin`: various built-in components
- - `builtin/nfs`: mount NFS shares
- - `builtin/nvme`: connect to NVMEoF devices using Linux kernel
- - `builtin/iscsiadm`: connect to iSCSI devices using `iscsiadm` commands
- `plugin`: delegate storage connection to external application
- - `plugin/http.json.dev`

`config` is an optional config for the mounter. Its content structure depends on mounter type.

---

When you create a new volume which is supposed to be used via iSCSI, for example, it does not automatically mean `builtin/iscsiadm` mounter.
Volume must set the mounter `id` to `iscsi`, and then node looks up its list of supported mounters.

In the example above `iscsi` mounter uses `plugin/http.json.dev` instead of `builtin/iscsiadm`.

There are no restrictions on the list of mounters.
You can configure several mounters for the same type of volumes, of completely disable some type by not listing it.
