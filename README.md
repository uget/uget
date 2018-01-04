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

## 2.2 Library usage

It's best to check out the [cli code](cli/commands.go) for examples.

Downloading a multitude of links:

```go
// First, get your links from somewhere:
urls := ...
// Then, create a new downloader:
downloader := core.NewClient()
// Add those links to the downloader's queue:
waitGroup := downloader.AddURLs(urls)
// Register some callbacks:
downloader.OnDownload(func(download *core.Download) {
	// Access the File field:
	download.File.Name()
	download.File.URL()
	download.File.Length()

	// hashObject is a hash.Hash used for generating a checksum
	checksum, algorithmName, hashObject := download.File.Checksum()

	// the provider, e.g. basic / imgur.com / uploaded.net / oboom.com etc.
	// see a list of all providers at https://github.com/uget/providers
	download.File.Provider()

	// wait for download to finish:
	download.Wait()
	// and get the error if there was one:
	download.Err()

	// OR: print download status every second

	interval := 1*time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("%s: started\n", download.File.Name())
	for {
		select {
		case <-ticker.C:
			percentage := download.Progress() / download.File.Size()
			fmt.Printf("  %s: %.2f%% of %d\n", download.File.Name(), percentage, download.File.Size())
		case <-download.Waiter():
			if download.Err() != nil {
				fmt.Printf("  %s: ERROR! %v\n", download.File.Name(), download.Err())
			} else {
				fmt.Printf("  %s: DONE!\n", download.File.Name())
			}
			return
		}
	}
})
// Start client (in the background)
downloader.Start()

// Wait for the jobs provided earlier to finish
waitGroup.Wait()

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
