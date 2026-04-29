# static Plugin

`static` is the built-in plugin that provides simple frontend and target implementations. It is useful for local development, simple always-on routing behavior, and command-backed targets.

## Exported Kinds

Frontends:

- `static:always`: periodically warms its target while the frontend is running.
- `static:http`: serves configured HTTP content and warms its target when requests arrive.

Targets:

- `static:cmd`: runs a shell command while the target is warm.
- `static:none`: no-op target handler.

There is no plugin-level configuration for `static`. Its top-level plugin config is ignored; behavior is configured on each frontend or target where options are documented below.

## static:always Frontend

`static:always` emits a warm signal once per second while started. It does not bind its own network listener; the core engine still installs the configured DNAT mapping from `listen` to the selected target endpoint.

Example:

```yaml
frontends:
  - name: public-http
    kind: static:always
    protocol: tcp
    listen: 0.0.0.0:8088
    flow_timeout: 1m
    target: backend:http
```

There are no configurable options for this frontend.

## static:http Frontend

`static:http` starts an HTTP server at the configured `listen` address. It serves static text at the configured endpoint path and emits a warm signal for each request.

Example:

```yaml
frontends:
  - name: warm-page
    kind: static:http
    protocol: tcp
    listen: 127.0.0.1:8090
    flow_timeout: 1m
    target: backend:http
    options:
      endpoint: /
      content: "warming backend\n"
```

Options:

- `content`: required string response body.
- `endpoint`: optional path; defaults to `/` and must start with `/`.

`static:http` requires `protocol: tcp`.

## static:cmd Target

`static:cmd` starts a command when the target warms and interrupts it when the target drains. If the process does not stop within five seconds, it is killed.

The command runs through:

```sh
/bin/sh -c "<command>"
```

Example:

```yaml
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
```

Options:

- `command`: required non-empty shell command string.

## static:none Target

`static:none` accepts warm, drain, and close transitions without doing any external work. It is useful when the backend already exists or when tests need a target implementation without process management.

Example:

```yaml
targets:
  - name: existing
    kind: static:none
    idle_timeout: 5m
    endpoints:
      - name: http
        protocol: tcp
        address: 127.0.0.1:8080
```

There are no configurable options for this target.

## Full Example

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
