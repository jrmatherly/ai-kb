# Cloud image maintenance scripts

Script scope, image-building notes and first-boot troubleshooting live in the [website-docs development guide](../../website-docs/06-development/01-dev-guide.md#cloud-image-scripts). For a normal install, see [Installation](../../website-docs/01-getting-started/02-installation.md).

This directory holds `prepare.sh`, `cleanup.sh`, `firstboot.sh` and their systemd units. `cleanup.sh` needs root and a typed `YES`. It wipes the build machine's data, logs, SSH authorizations, shell history and caches, lists any leftover key files (`id_rsa*`, `*.pem`, `*.key`) in `/tmp/cleanup-secrets-found.txt` for you to check, then powers it off. Run it only on a dedicated build machine. The scripts are the reference for arguments and actual behavior.
