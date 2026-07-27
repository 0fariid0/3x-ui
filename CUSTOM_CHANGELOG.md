# Custom Changelog

This repository is an independently maintained modified version of 3x-ui.

Base version: v3.5.0
Repository: https://github.com/0fariid0/3x-ui

## v3.5.1

- Added a per-Host subscription config name.
- Made the Host config name optional.
- Empty Host names fall back to the inbound/default config name.
- Preserved explicitly configured `{{INBOUND}}` + `{{HOST}}` templates.
- Redirected installer, updater, panel update checks, and repository links to `0fariid0/3x-ui`.
- Configured tagged GitHub Actions builds as stable releases.
- Configured Docker publishing to `ghcr.io/0fariid0/3x-ui`.
