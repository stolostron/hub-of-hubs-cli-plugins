# hub-of-hubs-cli-plugins
Kubectl plugins for hub-of-hubs

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