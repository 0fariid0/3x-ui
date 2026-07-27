# Custom Changelog

This repository is an independently maintained modified version of 3x-ui.

Base version: v3.5.0
Repository: https://github.com/0fariid0/3x-ui

## v3.5.5

- Subscription links shown in the panel now contain the actual URL-encoded client email in `?name=...` instead of the literal word `email`.
- Raw, JSON, and Clash subscription endpoints accept that exact name value and apply it consistently to every generated config.
- Legacy `?name=email` links remain supported.
- Public subscription-page copy links also use the owning client's actual email when it is known.
- Improved updater downloads with retries, archive validation, and an automatic IPv4 fallback for VPS networks whose GitHub release CDN route over IPv6 is broken.

## v3.5.4

- Added configurable Xray health monitoring with confirmed-failure detection.
- Added automatic Xray recovery only after a real crash; manual and intentional restart windows are ignored.
- Added restart cooldown and a rolling-window circuit breaker to prevent restart loops.
- Aligned scheduled restarts to real clock boundaries (for example :00/:15/:30/:45 and full hours).
- Clicking a QR code now copies its represented link instead of downloading the image.
- Subscription URLs now include `?name=email`; raw, JSON, and Clash outputs can name every config with the client email.
- Host addresses are stored and displayed without an embedded port. The group port applies to every address, while port `0` inherits the inbound port.
- Added a separate WS/HTTPUpgrade/XHTTP Host header for every IP or domain in a Host group.
- Updated generated OpenAPI files and tests so the custom Host behavior and new Xray settings are covered by GitHub Actions.

## v3.5.3

- Removed the @XrayUI Telegram link from the dashboard.
- Removed the Sanaei documentation and donation buttons from the sidebar.
- Added configurable scheduled restarts under Xray Settings > Basic.
- Supports minute, hour, and day intervals.
- Restarts Xray Core by default, with an optional full x-ui panel restart.

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
