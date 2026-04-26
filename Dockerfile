FROM archlinux:latest AS base

RUN pacman -Syu --noconfirm go=2:1.25.0-1 \
    && pacman -Scc --noconfirm

FROM base AS devcontainer

RUN pacman -Syu --noconfirm nftables \
    && pacman -Scc --noconfirm
