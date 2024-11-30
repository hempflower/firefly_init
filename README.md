# Firefly Init - MicroVM init

## Introduction

This is a tiny init for MicroVMs. It can set hostname, dns, and run a entrypoint command like docker.

## Build

This project is built with [Taskfile](https://taskfile.dev/). But you can also build it manually.

### Taskfile

```bash
task initramfs
```

### Manual Build

```bash
mkdir build
cd firefly
export CGO_ENABLED=0
go build -o ../build/init . 
```

then create initramfs:

```bash
mkdir build/initramfs
cp build/init build/initramfs/init
chmod +x build/initramfs/init
find . | cpio -o -H newc > ../initramfs.cpio
```

## Usage

FireFly read commandline arguments from `/proc/cmdline`:

for example:

```
root=/dev/vda ip=192.168.10.2/24:192.168.10.1 endpoint=/bin/bash
```

- `root` is the root device
- `ip` is the static ip 192.168.10.2/24 with gateway 192.168.10.1
- `endpoint` is the entrypoint command

## Features

- Set hostname
- Set dns
- Set static ip
- Run entrypoint
- Auto mount necessary filesystems such as /dev, /proc, /sys, etc
- Shutdown VM if entrypoint exits

## Supported Commandline Arguments

- `hostname` is the hostname
- `dns` is the dns server
- `root` is the root device
- `rootfstype` is the root filesystem type, default is `ext4`
- `ip` is the static ip
- `endpoint` is the entrypoint


## Rescue Shell

Firefly can run a rescue shell if something goes wrong. Such as cannot found root device, entrypoint not found...

You can add shell binary to `/bin/sh`, or run `task build-busybox` to add busybox to rootfs.

## License

MIT