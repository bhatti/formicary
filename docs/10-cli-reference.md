# Guide: CLI Reference

The `formicary` binary is the command-line interface (CLI) for starting and interacting with both Queen and Ant services.

## Global Flags

These flags can be used with the main command and all subcommands.

| Flag | Description | Default |
|---|---|---|
| `--config` | Path to a configuration file. | `$HOME/.formicary.yaml` |
| `--id` | A unique identifier for this instance. | `default-formicary` |
| `--port` | The HTTP port for the service to listen on. | `7777` |

### Configuration Precedence

Formicary loads its configuration from multiple sources, with the following order of precedence (1 is highest):
1.  Command-line flags (e.g., `--port 8080`)
2.  Environment variables (e.g., `COMMON_HTTP_PORT=8080`)
3.  Configuration file specified by `--config`.
4.  Configuration file at `$HOME/.formicary.yaml`.
5.  Default values coded into the application.

*Note: Environment variables should be prefixed. For nested keys in the YAML file like `common.http_port`, the corresponding environment variable is `COMMON_HTTP_PORT`.*

---

## `formicary queen` (or `formicary`)

Starts the Formicary Queen server. Running `formicary` without a subcommand is equivalent to running `formicary queen`.

The Queen is the leader node responsible for scheduling jobs, managing workers, and serving the API and dashboard.

### Usage
```bash
# Start the Queen using the default config file
formicary queen --id queen-main

# Start with a specific config and port
formicary --config /etc/formicary/config.yaml --port 80
```

### Queen-Specific Flags

| Flag | Description | Default |
|---|---|---|
| `--ant-tags` | Comma-separated list of tags for the embedded Ant worker. | `embedded,default` |
| `--ant-methods` | Comma-separated list of methods for the embedded Ant worker (e.g., `DOCKER,SHELL`). | `KUBERNETES,SHELL,HTTP_POST_JSON` |

These flags allow the Queen server to also act as an Ant worker, which is very useful for all-in-one deployments and local testing.

---

## `formicary ant`

Starts a standalone Formicary Ant worker. The Ant registers with the Queen and executes tasks.

### Usage
```bash
# Start an Ant that can run Docker and Shell tasks
formicary ant --id docker-ant-01 --tags "docker,linux"

# Start an Ant with a specific port for its own health/metrics endpoint
formicary ant --id ant-prod-02 --port 5555
```

### Ant-Specific Flags

| Flag | Shorthand | Description | Default |
|---|---|---|---|
| `--tags` | | Comma-separated list of tags this Ant supports. Tasks with matching tags will be routed here. | (none) |

---

## `formicary version`

Prints the build version information for the binary.

### Usage
```bash
formicary version
```

### Version Flags

| Flag | Shorthand | Description | Default |
|---|---|---|---|
| `--short` | `-s` | Print just the version number. | `false` |

