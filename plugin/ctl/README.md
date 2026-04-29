# ctl Plugin

`ctl` is the built-in control plugin for Proxy Gateway. It opens a Unix-domain socket and serves IPC requests against the running engine.

## Configuration

Enable the plugin under `plugins.ctl`:

```yaml
plugins:
  ctl:
    socket: /run/proxygw.sock
    codec: json
```

Fields:

- `socket`: required Unix socket path.
- `codec`: optional codec name. Supported values are `json` and `cbor`; the default is `json`.

The daemon fails to start if `socket` is missing or `codec` is unknown.

## Client Usage

`proxygwctl` reads the same runtime config as the daemon and uses `plugins.ctl` to connect.

```sh
proxygwctl --config /etc/proxygw.yaml status
```

The status response is printed as formatted JSON.

## Status Response

The current server handles status requests. The response includes:

- `closed`: whether the engine is closed.
- `targets`: target name, kind, state, last error, and endpoint list.
- `frontends`: frontend name, kind, state, protocol, listen address, selected target endpoint, proxy address, and last error.

Example shape:

```json
{
  "closed": false,
  "targets": [
    {
      "name": "backend",
      "kind": "static:cmd",
      "state": "active",
      "endpoints": [
        {
          "name": "http",
          "protocol": "tcp",
          "address": "127.0.0.1:8080"
        }
      ]
    }
  ],
  "frontends": [
    {
      "name": "public-http",
      "kind": "static:always",
      "state": "running",
      "protocol": "tcp",
      "listen": "0.0.0.0:8088",
      "target_name": "backend",
      "endpoint_name": "http",
      "proxyaddress": "127.0.0.1:8080"
    }
  ]
}
```

## Implementation Notes

The plugin registers itself as `ctl` in `init.go`. On load it creates an IPC listener with `ipc.Listen("unix", socket, codec)` and serves each connection with `Server.Serve`.

The IPC package supports request/response packets, notifications, JSON encoding, and CBOR encoding. Method IDs live under `plugin/ctl/ipc/method`.
