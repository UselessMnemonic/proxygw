FROM archlinux:latest AS base

RUN pacman -Syu --noconfirm go && pacman -Scc --noconfirm
ENV GOROOT=/usr/lib/go

FROM base AS devcontainer

RUN pacman -Syu --noconfirm openssh git nftables make \
    && pacman -Scc --noconfirm
