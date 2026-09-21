# Silo - WIP

CLI wrapping incus go package to easily work with containers / VMs for agent sandoxes.

## Documentaiton

- https://linuxcontainers.org/incus/docs/main/
- https://pkg.go.dev/github.com/lxc/incus

## Dependencies

Install `golang` with `mise use --global go`, which tracks `@latest` by default, and
`sudo apt install incus zfsutils-linux`.

## Setup

Need incus-admin, `sudo adduser "$USER" incus-admin`, then log out and back in.
Run `incus admin init` and follow these options:

```text
Would you like to use clustering? (yes/no) [default=no]: no
Do you want to configure a new storage pool? (yes/no) [default=yes]: yes
Name of the new storage pool [default=default]:
Name of the storage backend to use (dir, zfs) [default=zfs]:
Create a new ZFS pool? (yes/no) [default=yes]:
Would you like to use an existing empty block device (e.g. a disk or partition)? (yes/no) [default=no]:
Size in GiB of the new loop device (1GiB minimum) [default=30GiB]:
Would you like to create a new local network bridge? (yes/no) [default=yes]:
What should the new bridge be called? [default=incusbr0]:
What IPv4 address should be used? (CIDR subnet notation, “auto” or “none”) [default=auto]:
What IPv6 address should be used? (CIDR subnet notation, “auto” or “none”) [default=auto]: none
Would you like the server to be available over the network? (yes/no) [default=no]:
Would you like stale cached images to be updated automatically? (yes/no) [default=yes]:
Would you like a YAML "init" preseed to be printed? (yes/no) [default=no]: yes
```

```yaml
config: {}
networks:
- config:
    ipv4.address: auto
    ipv6.address: none
  description: ""
  name: incusbr0
  type: ""
  project: default
storage_pools:
- config:
    size: 30GiB
  description: ""
  name: default
  driver: zfs
storage_volumes: []
profiles:
- config: {}
  description: ""
  devices:
    eth0:
      name: eth0
      network: incusbr0
      type: nic
    root:
      path: /
      pool: default
      type: disk
  name: default
  project: default
projects: []
certificates: []
cluster: null
```

## SSH Usage

SSH client options can be passed after `--`:

```sh
silo ssh <instance> -- -A -L 3000:localhost:3000
```

These values are inserted before the generated SSH destination, so remote command
arguments are not supported.
