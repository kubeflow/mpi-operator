FROM debian:buster as builder

RUN apt update \
    && apt install -y --no-install-recommends \
        g++ \
        libopenmpi-dev \
    && rm -rf /var/lib/apt/lists/*
