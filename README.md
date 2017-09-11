# docker-volume-sheepdog


Docker Volume plugin to create persistent volumes in a [sheepdog](http://sheepdog.github.io/sheepdog/) cluster.

The driver is based on [the Docker Volume Plugin framework](https://docs.docker.com/engine/extend/plugins_volume/) and it integrates sheepdog into the Docker ecosystem by automatically creating a iSCSI storage volume([tgt](http://stgt.sourceforge.net/)) to a sheepdog vdi and making the volume available to Docker containers running.



## Preconditions

- sheepdog cluster has to be set up and running
- install and start required service and software

## System Requirements

- Docker Engine: 1.13.0+
- xfsprogs (`mkfs.xfs` command)
- iscsi-initiator-utils (`iscsiadm` command)
- scsi-target-utils (`tgtadm` command)
- sheepdog (`dog` command)

## Usage

First create a volume:

```
$ docker volume create -d sheepdog vol1
```

In this case, it will be created with default volume size (`DefaultVolSz` is be used.)


If you want to specify it explicitly, can use the `-o size=` option.
The syntax is equivalent to the `dog` command.(`10G`, `10M` ...)

```
$ docker volume create -d sheepdog vol1 -o size=12G
```

Then use the volume by passing the name (`vol1`):

```
$ docker run -it -v vol1:/data docker.io/alpine sh
```

List the volume:

```
$ docker volume list
DRIVER              VOLUME NAME
sheepdog            vol1
```

Inspect the volume:

```json
$ docker volume inspect vol1
[
    {
        "Driver": "sheepdog",
        "Labels": {},
        "Mountpoint": "/mnt/sheepdog/vol1",
        "Name": "vol1",
        "Options": {},
        "Scope": "global"
    }
]
```

Remove the volume:

```
$ docker volume rm vol1
```

