UGET (Universal GET)
====================

# Table of contents

1. Introduction
2. Getting started
  1. Installation
  2. Code examples
  3. CLI
3. Contributing
4. Reporting bugs

-------------------

# 1. Introduction

This project aims at providing an API / CLI for downloading remote files,
focusing mainly on premium file-hosters.

This repository holds the core project and aims to be very flexible.
Check out the supported providers at the [other repository](https://github.com/uget/providers)
 
**WARNING: This package is under heavy development, so documentation may fall behind and the APIs may change.**

# 2. Getting started

## 2.1 Installation

It's simple! Install Go, setup your `$GOPATH` and run:  
`go install github.com/uget/uget`

## 2.2 Code examples

It's best to check out the [cli code](cli/commands.go) for examples.

Downloading a multitude of links:

```go
// First, get your links from somewhere:
links := ...
// Then, create a new Downloader:
client := core.NewDownloader()
// Add those links to the downloader's queue (priority 1):
client.Queue.AddLinks(links, 1)
// Register some callbacks:
client.OnDownload(func(d *Download) {
  // use download for something, e.g.
  d.OnDone(func(d time.Duration, err error) {
    if err != nil {
      // download failed. Handle error.
      return
    }
    // Download finished.
  })
})
// Start downloader (synchronously, blocking).
client.StartSync()
// No downloads left, all jobs done.
```

## 2.3 CLI

### Implemented

Get remote files:
```bash
uget get CONTAINER_SPEC...
```

Resolve remote files:
```bash
uget resolve CONTAINER_SPEC...
```

`CONTAINER_SPEC` can be a plain file with a list of URLs.
If option `-i` is passed, the arguments are interpreted as direct URLs instead.

Add an account to a provider. You will be prompted for your credentials.
```bash
uget accounts add [PROVIDER]
```

List your saved accounts.
```bash
uget accounts list [PROVIDER]
```

### Not (fully) implemented yet

Start server as daemon.
```bash
uget daemon
```

Start server in foreground.
```bash
uget server
```

Push a list of files to the listening server.
```bash
uget push [OPTIONS...] CONTAINER_SPEC...
```

Tell the daemon to drop a container (or a file)
```bash
uget drop [ID]
```

Pause the daemon.
```
uget pause [--soft]
```

Continue the daemon.
```
uget continue
```

List the downloads.
```
uget list [CONTAINER_ID]
```

# 3. Contributing

Contributions are welcome! Fork -> Push -> Pull request.

# 4. Bug report / suggestions

Just create an issue! I will try to reply as soon as possible.
