# hub-of-hubs-cli-plugins
Kubectl plugins for Hub-of-Hubs

[![Go Report Card](https://goreportcard.com/badge/github.com/stolostron/hub-of-hubs-cli-plugins)](https://goreportcard.com/report/github.com/stolostron/hub-of-hubs-cli-plugins)
[![Go Reference](https://pkg.go.dev/badge/github.com/stolostron/hub-of-hubs-cli-plugins.svg)](https://pkg.go.dev/github.com/stolostron/hub-of-hubs-cli-plugins)
[![License](https://img.shields.io/github/license/stolostron/hub-of-hubs-cli-plugins)](/LICENSE)

## Build

```
$ make
```

## Run

1. During development, add the `bin` directory to the `PATH` environment variable.

   ```
   export PATH=${PWD}/bin:$PATH
   ```

   Alternatively, copy the executables from the `bin` directory to some directory in your path, e.g.:

   ```
   cp bin/* /usr/local/bin
   ```

2. Run:

   ```
   kubectl mcl
   ```
