<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/proxygw-logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/proxygw-logo.svg">
    <img alt="Proxy Gateway logo" src="assets/proxygw-logo.svg" width="260">
  </picture>
</p>

# Proxy Gateway

Proxy Gateway is a Linux gateway daemon that warms backend targets on demand and routes frontend traffic to those targets with nftables DNAT rules. It is built around a small plugin system: plugins register frontend kinds, target kinds, and optional lifecycle hooks, while the core engine manages state transitions, dataplane mappings, and shutdown.

The repository builds two binaries:

- `proxygw`: the daemon.
- `proxygwctl`: a control client for daemon status.

## Current Capabilities

- YAML runtime configuration with schema files in `configs/`.
- Namespaced plugin kinds, such as `static:always`, `static:http`, `static:cmd`, and `static:none`.
- Built-in control plugin (`ctl`) exposing a Unix-socket IPC endpoint.
- Built-in static plugin (`static`) for simple warm triggers and local command targets.
- nftables-based IPv4 and IPv6 DNAT mappings.
- conntrack timeout handling so idle targets can drain after configured flow timeouts.
- JSON and CBOR IPC codecs.

## Platform Requirements

This project is designed for Linux. The dataplane opens conntrack and nftables sockets and normally needs root or equivalent network administration capabilities.

Development and tests use Go as declared in `go.mod`.

## Build

```sh
make all
```

This writes binaries to `build/` by default:

```sh
build/proxygw
build/proxygwctl
```

The daemon build runs `go generate ./cmd/proxygw`. Generation reads `plugin.yaml` and creates `cmd/proxygw/plugin.go`, which blank-imports the configured plugins so their `init` functions register them.

Useful targets:

```sh
make proxygw
make proxygwctl
make test
make clean
```

The Makefile defaults to `GOOS=linux` and `GOARCH=amd64`.

## Run

```sh
sudo build/proxygw --config /etc/proxygw.yaml
```

The default config path is `/etc/proxygw.yaml`.

A systemd unit template is provided at `init/systemd/proxygw.service`.

## Control Client

`proxygwctl` reads the same config file as the daemon so it can find `plugins.ctl.socket` and `plugins.ctl.codec`.

```sh
build/proxygwctl --config /etc/proxygw.yaml status
```

`status` prints JSON containing engine state, targets, frontends, endpoint addresses, lifecycle states, and last errors.

## Configuration

The runtime config schema is in `configs/proxygw.schema.yaml`. The supported version is `v1`.

Minimal example:

```yaml
version: v1

log:
  output: stderr
  level: INFO

plugins:
  ctl:
    socket: /run/proxygw.sock
    codec: json

targets:
  - name: backend
    kind: static:cmd
    idle_timeout: 5m
    endpoints:
      - name: http
        protocol: tcp
        address: 127.0.0.1:8080
    options:
      command: "python3 -m http.server 8080 --bind 127.0.0.1"

frontends:
  - name: public-http
    kind: static:always
    protocol: tcp
    listen: 0.0.0.0:8088
    flow_timeout: 1m
    target: backend:http
```

Top-level fields:

- `version`: must be `v1`.
- `log.output`: `stdout`, `stderr`, or a filesystem path.
- `log.level`: parsed by Go `log/slog`; typical values are `DEBUG`, `INFO`, `WARN`, and `ERROR`.
- `plugins`: plugin-specific configuration keyed by plugin name.
- `targets`: backend resources that can be warmed and drained.
- `frontends`: listening or signaling resources that bind client traffic to target endpoints.

References use `namespace:name` text. Plugin kinds use their plugin namespace, for example `static:cmd`. Frontend `target` references use target and endpoint names, for example `backend:http`.

Durations use Go duration syntax, such as `30s`, `5m`, or `1h`.

Addresses use Go `netip.AddrPort` syntax, such as `127.0.0.1:8080`, `0.0.0.0:80`, or `[::1]:8080`.

## Runtime Model

The daemon loads config, constructs an engine, loads registered plugins, and asks each plugin to fill a namespace of frontend and target constructors. The engine then creates targets before frontends.

A target owns a DNAT group and a target handler. When a target is warmed, the handler starts or prepares the backend and the DNAT group is enabled. When the dataplane reports idle timeout, the target drains and DNAT is disabled.

A frontend owns a frontend handler and a DNAT mapping from its configured `listen` address to the selected target endpoint. Frontend handlers can emit warm signals through `ShouldWarm`; the engine forwards those signals to the target.

## Plugins

Build-time plugins are selected and namespaced in `plugin.yaml`:

```yaml
plugins:
  ctl: ctl
  static: static
  github.com/UselessMnemonic/proxygw-aws: aws
```

The `plugins` mapping is read in file order. For built-in plugins, the key must match a directory under `plugin/`. Non-built-in keys are treated as import paths by the generator. The value is the namespace used by runtime config references such as `static:http` or `aws:ec2`.

Plugin authors register handlers with `plugin.Register(source, handler)`, where `source` matches the `plugin.yaml` key. A plugin may define:

- `OnLoad(config, engine, namespace)`: receives plugin config, the engine, and a namespace to populate.
- `OnUnload()`: called during daemon shutdown.

Frontend and target handler interfaces live in `pkg/frontend/handler.go` and `pkg/target/handler.go`.

## Repository Layout

- `cmd/proxygw`: daemon entrypoint.
- `cmd/proxygwctl`: control client.
- `cmd/proxygw/gen`: plugin import generator.
- `configs`: JSON schemas for runtime and plugin config.
- `pkg/config`: typed YAML config model and validation.
- `pkg/dataplane`: nftables, conntrack, DNAT, and timeout handling.
- `pkg/engine`: top-level orchestration.
- `pkg/frontend`: managed frontend lifecycle.
- `pkg/target`: managed target lifecycle.
- `plugin/ctl`: built-in control IPC plugin.
- `plugin/static`: built-in static frontend and target plugin.
- `init/systemd`: systemd service template.

## Known Issues / TODOs
- Conntrack is currently polled rather than subcribed to, therefore use a sufficiently large timeout for any target.
- Regarding the above, ProxyGW works best for applications with long flows.
- There is no support for the `output` chain, therefore local traffic is never proxied
