# docker-volume-sheepdog

[![TravisCI](https://travis-ci.org/kazuhisya/docker-volume-sheepdog.svg)](https://travis-ci.org/kazuhisya/docker-volume-sheepdog)
[![Go Report Card](https://goreportcard.com/badge/github.com/kazuhisya/docker-volume-sheepdog)](https://goreportcard.com/report/github.com/kazuhisya/docker-volume-sheepdog)

Docker Volume plugin to create persistent volumes in a [sheepdog](http://sheepdog.github.io/sheepdog/) cluster.

The driver is based on [the Docker Volume Plugin framework](https://docs.docker.com/engine/extend/plugins_volume/) and it integrates sheepdog into the Docker ecosystem by automatically creating a iSCSI storage volume([tgt](http://stgt.sourceforge.net/)) to a sheepdog vdi and making the volume available to Docker containers running.


## Usage

First create a volume:

```
$ docker volume create -d sheepdog vol1
```

In this case, it will be created with default volume size (`DefaultVolSz` is be used. e.g. `10G`)


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

## Install

### Preconditions

- sheepdog cluster has to be set up and running
- install and start required service and software

### System Requirements

- Docker Engine: 1.13.0+
- `sudo` command
- xfsprogs (`mkfs.xfs` command)
- iscsi-initiator-utils (`iscsiadm` command)
- scsi-target-utils (`tgtadm` command)
- sheepdog (`dog` command)

### from distribution packages

A pre-built binary as well as `rpm` and `deb` packages are available from [the github release page](https://github.com/kazuhisya/docker-volume-sheepdog/releases).

Supported Distributions:

- RHEL based distributions 7.x (docker 1.13.x is provided in `docker-latest` package)
- Ubuntu 16.04 LTS (Xenial)


Then install and start the service:

```code
$ sudo yum install ./docker-volume-sheepdog-*.rpm
$ sudo systemctl start docker-volume-sheepdog
```

### from source

It uses [govendor](https://github.com/kardianos/govendor) to manage dependencies.

```code
$ go get -u github.com/kardianos/govendor
$ cd ${PATH-TO-GIT-REPO}
$ govendor sync
$ make
```

## Configuration file

Example `/etc/docker-volume-plugin.d/sheepdog.json` file:

```json
{
    "MountPoint": "/mnt/sheepdog",
    "DefaultVolSz": "10G",
    "VdiSuffix": "dvp"
}
```

Probably in most cases you will not need to change this setting. but if you need to change it, please check ours [wiki](https://github.com/kazuhisya/docker-volume-sheepdog/wiki/Full-Configuration).

## License

MIT, please see the LICENSE file.

## Disclaimer

This repository and all files that are included in this, there is no relationship at all with the upstream and vendor.
