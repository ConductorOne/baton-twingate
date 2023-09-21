![Baton Logo](./docs/images/baton-logo.png)

# `baton-twingate` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-twingate.svg)](https://pkg.go.dev/github.com/conductorone/baton-twingate) ![main ci](https://github.com/conductorone/baton-twingate/actions/workflows/main.yaml/badge.svg)

`baton-twingate` is a connector for Twingate built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Twingate API to sync data about groups, roles, and users.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-twingate
baton-twingate
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN=domain -e BATON_API_KEY=apiKey ghcr.io/conductorone/baton-twingate:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-twingate/cmd/baton-twingate@main

BATON_API_KEY=apiKey BATON_DOMAIN=domain
baton resources
```

# Data Model

`baton-twingate` will pull down information about the following Twingate resources:
- Groups
- Users
- Roles

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-twingate` Command Line Usage

```
baton-twingate

Usage:
  baton-twingate [flags]
  baton-twingate [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
  -f, --file string                         The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --customer-id string                  The api key for the twingate account. ($BATON_API_KEY)
      --domain string                       The domain for the twingate account. ($BATON_DOMAIN)
  -h, --help                                help for baton-twingate
      --log-format string                   The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                    The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -v, --version                             version for baton-twingate

Use "baton-twingate [command] --help" for more information about a command.

```
