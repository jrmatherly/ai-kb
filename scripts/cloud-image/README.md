# Cloud image maintenance scripts

Script scope, image-building notes and first-boot troubleshooting live in the [website-docs development guide](../../website-docs/06-development/01-dev-guide.md#cloud-image-scripts). For a normal install, see [Installation](../../website-docs/01-getting-started/02-installation.md).

This directory holds `prepare.sh`, `cleanup.sh`, `firstboot.sh` and their systemd units. `cleanup.sh` wipes the build machine's data, keys, SSH authorizations and caches, then powers it off. Run it only on a dedicated build machine. The scripts are the reference for arguments and actual behavior.
