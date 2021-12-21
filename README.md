# gogrok

[![Build Status](https://drone.meow.tf/api/badges/gogrok/gogrok/status.svg)](https://drone.meow.tf/gogrok/gogrok)

A simple, easy to use ngrok alternative (self hosted!)

The server and client can also be easily embedded into your applications, see the 'server' and 'client' directories.

Example usage
-------------

By default, the first time you run gogrok it'll generate both a server and a client certificate. These will be stored in ~/.gogrok, but can be overridden with the `gogrok.storageDir` option (or GOGROK_STORAGE_DIR environment variable)

Server:

`gogrok serve`

Client:

`gogrok --server=localhost:2222 http://localhost:3000`

Server
------

```
$ ./gogrok serve --help
Start the gogrok server

Usage:
  gogrok serve [flags]

Flags:
      --bind string       SSH Server Bind Address (default ":2222")
      --domains strings   Domains to use for
  -h, --help              help for serve
      --http string       HTTP Server Bind Address (default ":8080")
      --keys string       Authorized keys file to control access

Global Flags:
      --config string   config file (default is $HOME/.gogrok.yaml)
      --viper           use Viper for configuration (default true)
```

Client
------

```
$ ./gogrok client --help
Start the gogrok client

Usage:
  gogrok client [flags]

Flags:
  -h, --help                help for client
      --key string          Client key file
      --passphrase string   Client key passphrase
      --server string       Gogrok Server Address (default "localhost:2222")

Global Flags:
      --config string   config file (default is $HOME/.gogrok.yaml)
      --viper           use Viper for configuration (default true)
```

Docker
------

Example docker compose file. Caddy is suggested as a frontend using dns via cloudflare and DNS-01 for wildcards.

```yaml
version: '3.7'

services:
  gogrok:
    image: tystuyfzand/gogrok:latest
    ports:
    - 2222:2222
    - 8080:8080
    volumes:
    - /docker/gogrok/config:/config
    environment:
    - GOGROK_DOMAINS=gogrok.ccatss.dev
    - GOGROK_AUTHORIZED_KEY_FILE=/config/authorized_keys
```