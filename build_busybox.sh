#!/bin/bash

BUSYBOX_VERSION=1.37.0

cd build
if [ ! -f busybox-${BUSYBOX_VERSION}.tar.bz2 ]; then
    wget https://www.busybox.net/downloads/busybox-${BUSYBOX_VERSION}.tar.bz2
    tar xf busybox-${BUSYBOX_VERSION}.tar.bz2
fi
cd busybox-${BUSYBOX_VERSION}
cp ../../.config .config
make
make install

echo copy busybox to rootfs
# copy busybox to rootfs
cp _install/* ../initramfs/
