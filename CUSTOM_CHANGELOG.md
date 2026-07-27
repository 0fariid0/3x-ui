# Custom Changelog

This repository is an independently maintained modified version of 3x-ui.

Base version: v3.5.0
Repository: https://github.com/0fariid0/3x-ui

## v3.5.2

- Host config names are now complete per-Host templates and are never combined with another name.
- Added full per-client expansion of Host templates such as `{{EMAIL}}` and `{{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D`.
- Plain Host names are emitted exactly as entered.
- Empty Host names still fall back to the global/inbound default naming logic.
- Added quick-pick remark template presets to the global and per-Host name fields.
- Restored the WARP routing selector below IPv4 routing in Xray basic routing settings.

## v3.5.1

- Added a per-Host subscription config name.
- Made the Host config name optional.
- Empty Host names fall back to the inbound/default config name.
- Preserved explicitly configured `{{INBOUND}}` + `{{HOST}}` templates.
- Redirected installer, updater, panel update checks, and repository links to `0fariid0/3x-ui`.
- Configured tagged GitHub Actions builds as stable releases.
- Configured Docker publishing to `ghcr.io/0fariid0/3x-ui`.
