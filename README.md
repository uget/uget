# Universal Getter. Includes plain HTTP and several file-hosters.

Check out the supported providers at the [other repository](http://github.com/uget/providers)

## Usage

### CLI

Start a daemon that runs in the background and listens for download requests
```bash
ugetd
```

```bash
uget push [OPTIONS...] CONTAINER_SPEC [CONTAINER_SPEC...]
```

`CONTAINER_SPEC` can be one of:  
- plain file with a list of URLs
- json file with a list of file definitions and options
- plain URL

Tell the daemon to drop a container (or a file)
```bash
uget drop [ID]
```

Pause the daemon. `--force` forces a pause.
For file hosters that don't support partial GETs,
the download speed will usually only be strongly restrained.
```
uget pause [--force]
```

Continue the daemon.
```
uget continue
```

List the downloads.
```
uget list [CONTAINER]
```

Resolve a container, list its content, etc.
```
uget meta CONTAINER_SPEC
```
