
```bash

midclt call sharing.nfs.create '{"path": "/mnt/test/nfs/"}'
midclt call sharing.nfs.query '[["path","=","/mnt/test/nfs/"]]'
midclt call sharing.nfs.delete '1'

midclt call iscsi.extent.create '{"name":"test","type":"DISK","disk":"zvol/test/zvol"}'
# Disk currently in use by extent test
midclt call iscsi.extent.query '[["disk","=","zvol/test/zvol"]]'
midclt call iscsi.extent.query '[["disk","=","zvol/test/prefix.truenas-manual-iscsi.02b62bad-b839-11f0-b0e3-bc2411f4ca2d.suffix"]]'
midclt call iscsi.extent.query '[["disk","=","zvol/test/prefix.truenas-manual-iscsi.61830b81-b839-11f0-ad27-bc2411f4ca2d.suffix"]]'

midclt call iscsi.target.create '{"name":"test-target","groups":[{"portal":1,"initiator":null,"authmethod":"NONE","auth":null}]}'
# Target with this name already exists
midclt call iscsi.target.query '[["name","=","test-target"]]'
midclt call iscsi.target.query '[["name","=","prefix.truenas-manual-iscsi.16427c4f-b83d-11f0-b12a-bc2411f4ca2d.suffix"]]'
midclt call iscsi.target.delete '3'

midclt call iscsi.targetextent.create '{"target":3,"extent":2}'
# Extent is already in use

midclt call iscsi.initiator.create '{"initiators":["iqn.test"]}'
midclt call iscsi.initiator.query '[["initiators","rin","iqn.test"]]'

```
