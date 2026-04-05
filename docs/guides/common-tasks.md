# Common tasks

## Add a new host

1. Add an entry under `hosts` in your resolved config file (`~/.ghr/runners.yml` by default)
2. Ensure SSH key-based access works: `ssh user@host true`
3. Add runner entries referencing the new host
4. Run `ghr setup && ghr up`

## Scale up

Change `count` in your runners YAML, then:

```bash
ghr setup   # configures new instances
ghr up      # starts them
```

## Update runner version

```bash
ghr update
```

This removes existing runners, downloads the latest runner binary, reconfigures, and starts them.

## Clean up ghost runners

```bash
ghr cleanup
```
