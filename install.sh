#!/bin/bash

apt-get update

apt-get install -y \
    git \
    cgroup-lite \
    htop \
    tmux \
    build-essential \
    protobuf-c-compiler \
    libprotobuf-c0-dev \
    libprotobuf-dev \
    bsdmainutils \
    protobuf-compiler \
    python-minimal \
    libaio-dev \
    iptables \
    git-core \
    libtool 

cd

git clone https://github.com/xemul/criu.git

(
    cd criu
    make
    make install-criu
)
