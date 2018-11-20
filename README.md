# zackup - Backup to ZFS

Small utility to replace BackupPC.

- for each `host` in `list_of_hosts` do:
  - if `zfs_dataset_for_host` does not exist:
    - create `zfs_dataset_for_host`
  - for each `command` in `list_of_pre_commands_for_host`:
    - execute `command` on host
  - rsync `list_of_configured_files` from `host` to `zfs_dataset_for_host`
  - for each `command` in `list_of_post_commands_for_host`:
    - execute `command` on `host`
  - create snapshot of `zfs_dataset_for_host`


## Usage

    zackup [COMMAND] [--root ROOT_DIR]

Instead of `--root`, you may set a `ZACKUP_ROOT` environment variable.
The command line flag takes precedence if both are given.

    export ZACKUP_ROOT=ROOT_DIR
    zackup [COMMAND]


### `COMMAND`

Defaults to `help`.

- `run`

  Creates a backup for each host config. Backups are stored locally in
  a per-host dataset.

  Run `zackup help run` for a list of possible options.

- `status`

  Prints a list of hosts and their backup status (last success, size)

- `help`

  Prints a help listing with all available commands.

You can run `zackup help COMMAND` to get a list of possible options.

### `ROOT_DIR`

Defaults to `/usr/local/etc/zackup`. Its content is used as config tree.
We assume these files exist in `ROOT_DIR`:

    ROOT_DIR/
    +-- config.yml                    service configuration
    +-- defaults.yml                  global defaults for host configs
    +-- hosts/
        +-- $host/config.yml          host config (variant A)
        +-- $host.yml                 host config (variant B)
        +-- $host/{pre,post}.*.sh     host-specific scripts (optional)

The *list of hosts* is comprised of each `ROOT_DIR/hosts/$host` entry.
A `$host` is a string matching the rules for DNS host name labels.

A host may have scripts executed (via SSH) *before* and/or *after*
rsyncing. These scripts are defined by the `ROOT_DIR/hosts/$host/pre.*.sh`
and `ROOT_DIR/hosts/$host/post.*.sh` files. See sec. "Hooks" below.

You may also create a `ROOT_DIR/hosts/example.com.yml` instead of a
`ROOT_DIR/hosts/example.com/config.yml` if you don't have any pre- or
post-commands to execute (i.e. you have no script *files*, you can still
define *inline* script one-liners).

It is an error to have both `ROOT_DIR/hosts/$host/config.yml` and
`ROOT_DIR/$host.yml`.


## Setup

It is recommended to create a compressed ZFS dataset for all backups:

```console
# zfs create -o compression=lz4 zpool/zackup
```

and add `$zpool/zackup` as `root_dataset` to the service configuration
file.


## Service config

zackup requires a previously mentioned service configuration file in
`ROOT_DIR/config.yml`, which defines these properties:

```yaml
parallel:     uint8     # number backups to run in parallel
root_dataset: string    # base dataset to create host-datasets under
mount_base:   path      # working directory to mount host dataset into
log_level:    enum      # one of DEBUG, INFO, WARN, ERROR, FATAL, PANIC (case insensitive)
graylog:      addr      # if set, write logs to this GELF UDP endpoint
```

The defaults are:

```yaml
parallel:     5
root_dataset: zpool
mount_base:   /var/zackup
log_level:    info
```


## Hooks (pre- and post-scripts)

Within the host config directory, you can define `pre.*.sh` and `post.*.sh`
files, which are executed in alphabetically order *before* the rsync
process starts, and *after* it has finished.

For conveniance, you can add *inline* hook scripts into the host config
file (more on that in the next section). These inline scripts are executed
before any `pre.*.sh` or `post.*.sh` scripts.

Please note that hook scripts (both inline and files) are piped directly
into `/bin/sh -esx`, so you don't need a shebang. Think of a simple
`cat $host/pre.*.sh | ssh $host /bin/sh -esx`.

If any of those scripts exits with a non-zero exit status, the backup is
marked as failed.


## Host config

A host's config file is written in YAML and has this structure:

```yaml
ssh:
  user: string        # username on the remote host
  port: uint16        # SSH port number
  identity_file: path # path (on localhost) to SSH identity file (private key)

rsync:
  included: []string  # rsync pattern for included files/directories
  excluded: []string  # rsync pattern for excluded files/directories
  args:     []string  # other rsync arguments

# Inline scripts executed on the remote host before and after rsyncing,
# and before any `pre.*.sh` and/or `post.*.sh` scripts for this host.
pre_script:  string
post_script: string
```


## Global config

zackup looks for a global config file in `ROOT_DIR/global.yml`.

Use this file to specify defaults (a single host's config is basically
merged into the global config). zackup brings no defaults (not even for
rsync!), but you can use this as a start:

```yaml
ssh:
  user: root
  port: 22
  identity_file: /etc/zackup/id_rsa.pub

rsync:
  included:
  - /etc
  - /home
  - /opt
  - /root
  - /srv
  - /usr/local/etc
  - /var/spool/cron
  - /var/www
  excluded:
  - tmp
  - *.log
  - *.log.*
  - .cache
  - .config
  args:
  - --numeric-ids
  - --perms
  - --owner
  - --group
  - --devices
  - --specials
  - --links
  - --hard-links
  - --block-size=2048
  - --recursive

# pre_script:  # empty
# post_script: # empty
```
