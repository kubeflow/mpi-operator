FROM debian:buster as builder

RUN apt update \
    && apt install -y --no-install-recommends \
        g++ \
        libmpich-dev \
    && rm -rf /var/lib/apt/lists/*
