## v3.6.30

- Fixed the unstable v3.6.29 optimisation attempt by reverting the risky backend/runtime changes that could break release builds on some targets.
- Kept only safe panel-side load reductions: slower client-page polling, less frequent modal auto-refresh, no background refresh while the tab is hidden, and lower UI animation overhead.
- Lightly relaxed Xray/client scan traffic cadences to reduce panel CPU pressure without touching user traffic routing logic.

## v3.6.28

- Removed the generated `?name=` query parameter from normal subscription, JSON subscription, and Clash subscription links.
- Existing custom subscription URLs with stale `name` query parameters are sanitized before being shown, copied, or used in QR/deep-link flows.
- Frontend subscription copy/QR helpers now keep plain subscription URLs and preserve only non-name query parameters.


## v3.6.27
- Client usage details now show all currently online inbounds plus the three most recent previously used inbounds, with the last connection time for each inbound.
- The connected-inbounds panel stays inbound-based and does not expand multiple host/IP/share links for the same inbound.

# Custom changelog

## v3.6.25

- Fixed multi-inbound online attribution: a client is now shown online only on the exact inbound tags observed for that client, rather than combining independent per-email and per-inbound aggregate counters.
- Added live GUID -> email -> inbound-tag reporting and propagation across node chains, with legacy fallback only for older nodes that do not expose precise attribution.
- Reworked the client usage "Hosts" view into "Connected Inbounds" so multiple host/IP choices for one inbound are no longer all presented as used/online.
- Enabled a transient bounded runtime access log when no admin access log is configured; it is parsed in memory every 5 seconds for exact inbound attribution and truncated after ingestion.
- Updated panel, OpenAPI, and destination updater version identifiers to 3.6.25.

## v3.6.24

- Client usage/statistics modal now refreshes automatically every 5 seconds while it remains open.
- The first refresh still runs immediately; later refreshes are silent so the modal does not flash a loading spinner.
- Auto-refresh now updates the complete client insight report (usage totals/chart, IPs, apps, hosts, destinations, anomalies and events) instead of refreshing only the Destinations tab.
- Recursive timeout scheduling prevents background refresh requests from stacking on slow connections.
- Updated panel, OpenAPI, and destination updater version identifiers to 3.6.24.

## v3.6.23

- Added a configurable DNS resolver for generated Xray JSON subscriptions directly in Xray Settings.
- Changed the default generated Xray JSON subscription resolver from 8.8.8.8 to 1.1.1.1.
- Added Cloudflare, Cloudflare DoH, Google, and Quad9 presets while still allowing custom Xray-compatible resolver addresses.
- DNS changes are loaded per subscription request, so they take effect without restarting Xray or the panel.
- Kept raw Shadowsocks `ss://` links standards-compatible: client-side DNS cannot be forced through SIP002 and remains controlled by the importing app.
- Added backend validation, persistence coverage, and a concurrency-safe JSON template clone so changing DNS cannot mutate the shared subscription template.
- Updated panel, OpenAPI, and destination updater version identifiers to 3.6.23.

## v3.6.22

- Replaced wall-clock destination expiration with an exact rolling limit of five distinct one-minute activity buckets per client.
- The sixth activity minute removes only the oldest minute bucket; no timer removes data while the client is offline.
- Destination reports always read the newest five stored buckets independently of the selected 24-hour traffic range.
- Removed destination deletion from both the access-log ingestion timer and the general insight cleanup job.
- Added regression coverage for five buckets, sixth-minute rollover, offline persistence, and reports older than 24 hours.
- Updated the panel, OpenAPI, and destination updater version identifiers to 3.6.22.

## v3.6.21

- Fixed the Go build failure in client destination ingestion by removing the unused `now` local variable.
- Preserved all v3.6.20 live destination, mobile statistics, 24-hour defaults, chart color, and edit shortcut changes.
- Updated the panel, OpenAPI, and destination updater version identifiers to 3.6.21.

## v3.6.20

- Built directly from the v3.6.13 source baseline; v3.6.14 through v3.6.19 were intentionally excluded.
- Preserved the latest real five-minute destination snapshot for up to ten minutes after disconnect.
- Changed destination aggregation to minute buckets and required both client-online state and recent destination activity for the green indicator.
- Added the direct mobile statistics action and removed the duplicate overflow-menu entry.
- Removed the detailed report from Client Information so statistics are loaded only in the dedicated modal.
- Made the previous 24 hours the default across client reports and usage alerts.
- Made download the theme-primary chart series and upload the muted series.
- Added the existing client-edit action to the statistics header, including dashboard-to-client deep linking.

## v3.6.13

- Added the Neo Smart subscription usage-history endpoint for real 24-hour hourly and 30-day daily charts.
- Reused the panel's existing compact traffic rollups instead of adding duplicate history tables or collectors.
- Changed destination tracking to short-lived reporting: stale destinations are automatically removed after 10 minutes.
- Marked destinations seen in the last 2 minutes as active, sorted active rows first, and added background refresh while the destination tab is open.

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
