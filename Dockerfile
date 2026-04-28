FROM archlinux:latest AS base

RUN pacman -Syu --noconfirm go make && pacman -Scc --noconfirm
ENV GOROOT=/usr/lib/go

FROM base AS dev

RUN pacman -Syu --noconfirm \
    bash \
    curl \
    freetype2 \
    git \
    libxi \
    libxext \
    libxrender \
    libxtst \
    nftables \
    openssh \
    procps-ng \
    sudo \
    tar \
    unzip \
    which \
    && pacman -Scc --noconfirm
