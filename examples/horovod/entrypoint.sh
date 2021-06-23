#!/bin/sh

SSH_MNT=${SSH_MNT:-/mnt/ssh}
if [ -d "$SSH_MNT" ]; then
  mkdir -p $HOME/.ssh
  cp "${SSH_MNT}"/* $HOME/.ssh
  chmod 600 -R $HOME/.ssh
fi

exec "$@"