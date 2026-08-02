# Custom changelog

## v3.6.5

- Added configurable client-usage ranges from 24 hours to 365 days.
- Added a dedicated desktop usage-chart action for every client.
- Added a full client usage modal with upload/download charts, hourly pattern, totals, averages, peak minute/day/hour, IPs, apps, hosts, anomalies, and change history.
- Added a dashboard section for top consumers and abnormal-usage alerts with direct access to each client report.
- Added enriched report totals and a new `/panel/api/clients/usageAlerts` endpoint.

## v3.6.4

Built directly from the custom v3.6.0 source. Versions v3.6.1, v3.6.2, and v3.6.3 are not part of this release history.

- Added subscription maintenance mode for RAW, Xray JSON, and Clash/Mihomo outputs.
  - Notice mode prepends a temporary informational entry while keeping normal configurations.
  - Fallback mode can replace normal configurations with validated emergency links.
- Added durable per-client one-minute traffic buckets, recent IP history, application/OS detection, related Hosts, peak-hour analysis, and renewal/change history.
- Added configurable abnormal-usage detection for sudden traffic spikes, sustained high traffic, and possible subscription sharing.
- Added automatic reactions: alert only, temporary disable, or temporary transfer to a dedicated limited-speed inbound.
- Added cooldowns, history retention, reversible actions, rollback protection, and safeguards against overwriting later manual changes.
- Added a detailed client report endpoint and responsive report UI.
- Added SQLite/PostgreSQL migrations, OpenAPI schemas, Persian/English translations, and regression tests.

## v3.6.0

Based on the official 3x-ui v3.6.0 source while preserving the custom feature set maintained in this repository.

- Kept per-host subscription names, full template variables, presets, and exact fallback behavior.
- Kept per-client link visibility, disabled-host opt-in, per-address headers, and effective host port handling.
- Kept subscription application history with up to three recent applications.
- Kept the single always-first display-only subscription entry.
- Kept scheduled Xray/x-ui restarts, aligned schedules, Tehran/server clock, restart history, and delayed Xray follow-up restart.
- Kept Xray health monitoring and loop protection, disabled by default.
- Kept QR click-to-copy and client-specific subscription profile names.
- Kept WARP routing controls and removed upstream Telegram/documentation/donation shortcuts from the panel UI.
- Updated installer, updater, panel update checks, and releases to `0fariid0/3x-ui`.
- Removed the Docker GitHub Actions workflow.
