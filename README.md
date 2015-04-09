# Universal Getter. Includes plain HTTP and several file-hosters.

Check out the supported providers at the [other repository](http://github.com/uget/providers)

## Disclaimer

**This package is under heavy development, so documentation may fall behind and the APIs may change.**

## Usage

### CLI

#### Implemented

Get remote file. Plain HTTP is natively supported. Providers may be added (see above).
```bash
uget get LINK...
```

#### Not (fully) implemented yet

Start server as daemon.
```bash
uget daemon
```

Start server in foreground. (Currently lacks any usable features)
```bash
uget server
```

Push a list of files to the listening server. (Not implemented yet)
```bash
uget push [OPTIONS...] CONTAINER_SPEC [CONTAINER_SPEC...]
```

Add your credentials for a provider. You will be prompted.
```bash
uget accounts add PROVIDER
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
