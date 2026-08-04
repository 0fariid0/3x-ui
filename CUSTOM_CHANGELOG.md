## v3.6.16

- Preserve each client's newest five-minute destination window even after the client disconnects.
- When a client's retained observations span ten minutes, prune the older five-minute half instead of deleting by wall-clock age.
- Keep the green live marker limited to online clients with a destination seen in the last five minutes.
- Make 24 hours the backend/API default for client reports and dashboard usage alerts.
- Keep the direct mobile statistics action, dedicated report-only UI, download-first chart color, and in-report edit action from v3.6.15.
- Preserve the verified Linux amd64 release workflow and the frontend production-build fix from v3.6.15.

## v3.6.15

- Fix the production frontend build by importing `useSearchParams` from the installed `react-router` package instead of the absent `react-router-dom` package.
- Keep every functional change from v3.6.14 unchanged.
- Replace the fragile seven-architecture release matrix with one verified Linux amd64 release pipeline.
- Run frontend type-check, frontend production build and Go production build before publishing an asset.
- Verify the release archive and its SHA-256 before creating the GitHub Release.

## v3.6.14

- Keep destination observations for a rolling 10-minute grace window with 5-minute cleanup, so recent destinations remain visible after a client goes offline.
- Show the usage/report action directly on mobile client cards.
- Remove the duplicated detailed report from Client Information; reports now open only from the dedicated statistics action.
- Default client reports and dashboard usage alerts to the last 24 hours.
- Use the primary theme color for download and the secondary color for upload.
- Add a client edit action beside the client name inside the usage report modal.
- Fix duplicate JSX/declarations inherited by the v3.6.13 source package.

## v3.6.13

- Neo Smart subscription usage history API (24h / 30d) backed by existing compact rollups.
- Short-lived destination view with five-minute cleanup and green recent-activity indicators.

# Custom changelog

## v3.6.12

- Added a dedicated **NordVPN** service-routing selector beside the existing `warp` selector.
- NordVPN NordLynx outbounds now use the stable tag `NordVPN` instead of a server-dependent `nord-*` tag.
- Existing legacy `nord-*` outbounds are detected and migrate their routing rules when the NordVPN outbound is reset.
- Renamed the basic WARP routing label to the exact lowercase name `warp`.
- Kept only the tag-triggered release workflow to avoid unrelated failing checks on normal pushes.

## v3.6.11

- Added the complete bundled Google IPv4/IPv6 prefix snapshot from Google's official `goog.json` feed, including `142.250.0.0/15` for addresses such as `142.251.20.113`.
- Added automatic 12-hour Google prefix refreshes from the official Google JSON feed alongside Telegram and Meta updates.
- Provider refresh requests now run concurrently with independent timeouts, so one slow source cannot block the others.
- Bundled Telegram, Meta, and Google ranges are always checked before dynamic data, preventing incomplete refresh responses from removing known classifications.
- Destination reports normalize stored IP formats and reclassify existing `Other` rows immediately with the latest rules.

## v3.6.10

- Fixed stale destination rows remaining classified as `Other` after Telegram/Meta rules were added.
- Destination reports now reclassify stored rows using current domain/CIDR rules.
- Hourly destination upserts now refresh classification metadata.

## v3.6.9

- Added official Telegram IPv4/IPv6 CIDR recognition, including `149.154.160.0/20`.
- Added bundled Meta network prefixes so IP-only Meta traffic is no longer shown as `Other`.
- Added a background prefix updater: Telegram is refreshed from its published CIDR list and Meta prefixes are refreshed from current AS32934/AS63293 announcements.
- Kept domain matches authoritative: Instagram domains remain `Instagram`; IP-only Meta matches are labelled `Instagram / Meta` because Meta shares network ranges between Instagram, Facebook and WhatsApp.
- Added a distinct `network` confidence label in client destination reports.

## v3.6.8

- Added per-client destination tracking, disabled by default for every client.
- Added compact hourly destination aggregates with 14-day retention and bounded raw logging.
- Added service/domain/IP destination reports without storing URLs or content.

## v3.6.7

- Fixed the backend build failure in `ClientInsightService.GetReportForRange` by initializing the panel location before rollup migration uses it.
- Made OpenAPI generation read `internal/config/version`, preventing the CI codegen job from rewriting the committed API version to `3.x`.
- Regenerated the public OpenAPI document for v3.6.7.

## v3.6.6

- Unified all report, history, log, and chart timestamps with the panel-selected calendar and server/Tehran clock offset.
- Added rolling 12-hour and 24-hour client traffic charts with separate hourly upload/download data.
- Reworked top-consumer alerts into a compact text-only dashboard card and placed compact Xray/system uptime beside it.
- Added bounded minute/hour/day traffic rollups so long-range reports use compact summaries instead of unbounded minute records.
- Added hourly insight cleanup, SQLite WAL compaction, and strict small-file limits for panel, Xray, and IP-limit logs.

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
