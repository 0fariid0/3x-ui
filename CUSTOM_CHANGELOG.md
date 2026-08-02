# Fara Xray custom changelog

## v3.6.3 — Responsive interface redesign

- Rebuilt the shared application shell instead of applying another visual theme.
- Added a fixed desktop sidebar, compact tablet rail, mobile header and bottom navigation dock.
- Reworked global cards, tables, forms, tabs, modals and page spacing for smaller screens.
- Simplified page headers and dashboard presentation while preserving existing panel features.
- Preserved RTL, light, dark and ultra-dark modes.

## v3.6.2 — Complete Fara Xray interface rebuild

- Replaced the inherited panel navigation with a custom Fara Xray application shell, grouped workspace navigation, command search, desktop workspace bar, and mobile app drawer.
- Rebuilt the mobile experience with a compact top bar, thumb-friendly bottom dock, responsive forms, horizontally managed data tables, and touch-sized controls.
- Reorganized dashboard information into an operational bento layout with new service identity, circular resource gauges, network throughput blocks, and quick actions.
- Added a consistent section hero and visual identity across Dashboard, Inbounds, Clients, Groups, Nodes, Hosts, Settings, Xray, Outbounds, Routing, and API documentation.
- Rebuilt Settings and Xray pages around task-oriented local navigation and persistent action areas instead of the inherited long-form page layout.
- Replaced the login page with a split-screen Fara Xray experience and rebuilt the user subscription page around a status hero, QR area, usage summary, and clearer information hierarchy.
- Reworked cards, tables, forms, modals, tabs, statistics, alerts, spacing, and dark surfaces throughout the panel instead of applying a color-only theme.
- Preserved all v3.6.0 official fixes and every previously requested custom host, subscription, restart, clock, health-monitor, and visibility feature.

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
