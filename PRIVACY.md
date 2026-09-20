# Privacy Policy

_Last updated: 2026-09-20_

`disguise-mcp` is a self-hosted bridge that you run on your own machine or
network. It is designed to keep your data on your infrastructure.

## What data it handles

- **disguise API traffic.** The bridge forwards requests to, and returns
  responses from, the **disguise server you configure** (its Designer API). This
  is show/production data on your own network.
- **Configuration.** Your settings (disguise host/port, enabled API sections,
  bind addresses) are read from and written to a local YAML config file that you
  control.

## What it does not do

- It does **not** send your data to the author, to Anthropic, to OpenAI, or to
  any other third party.
- It contains **no telemetry, analytics, or tracking**.
- It only connects to the disguise server you configure. Its MCP endpoint and
  admin console bind where you tell them to (all interfaces by default, since it
  is meant to run on a show network; set them to `127.0.0.1` to restrict to the
  local machine).

## Data retention

The bridge persists nothing beyond your local configuration file. Uninstalling
the connector or deleting your config file removes everything it retains.

## Contact

Questions or issues: <https://github.com/medcelerate/disguise-mcp/issues>
