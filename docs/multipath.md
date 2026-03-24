
# Multipath setup

This file discusses how you could setup `multipath` daemon on client machines.

## Install multipath

```bash
# Debian and debian-based distros
sudo apt install multipath-tools

sudo systemctl enable multipathd
sudo systemctl restart multipathd
sudo systemctl status multipathd --no-pager

```

## Configure multipath

Multipath as a concept consists of several things:
- kernel module `multipath`
- service running `multipathd`
- configuration files for `multipathd`
- `multipath` and `multipathd` binaries as clients

Multipath does not fit CSI concept very well.
It was created as a standalone service that will watch all block devices in the system,
and automatically create corresponding virtual devices that can balance load between underlying devices,
and automatically disconnect failed devices,
and automatically manage partitions,
and handle all this on boot.

CSI delegates most of the automation to CO as a centralized controller.
We still need multipath to automatically handle device failures, but we don't need any of the other stuff.
We can disable device discovery and partition management in multipath config.

If you already have a working multipath setup, you should be able to continue using it without changes,
but if you are configuring multipath from scratch
then you are advised to use a minimalistic configuration from example down below:

```bash

 sudo tee /etc/multipath.conf << EOF
defaults {
  user_friendly_names no
  find_multipaths off
  skip_kpartx yes
  flush_on_last_del always
  auto_resize never
}
EOF

```

## Possibly helpful debug commands

```bash

# check out list of device ids which are allowed to be used in multipath
sudo cat /etc/multipath/wwids

# run interactive terminal
sudo multipathd -k
# the terminal does not have useful help, you need to find commands using man
man multipathd

# reload configuration
sudo multipathd -k'reconfigure'

# check status
sudo multipathd -k'show maps'
sudo multipathd -k'show topology'
sudo multipathd -k'show devices'

```

# Example how to manually test multipath

```bash

man multipath

# read id of specified block device, add the id into /etc/multipath/wwids
# this will also print the id
sudo multipath -a /dev/sdb
# ask multipathd to reload config
sudo multipath -r
# check current status
sudo multipath -l
# remove specified id from /etc/multipath/wwids
sudo multipath -w 36001405668e5fcff5374e6ab681fa76b
# ask multipathd to delete multipath block device with specified id
sudo multipath -f 36001405668e5fcff5374e6ab681fa76b
# cleanup /etc/multipath/wwids, remove currently unused entries
sudo multipath -W

multipathd resize map

```
