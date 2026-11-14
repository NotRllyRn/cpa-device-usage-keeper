# Working on device usage

- Keep upstream imports and default behavior intact. Prefer small, localized,
  optional capabilities enabled by device adapters. Do not copy charts or fork
  stylesheets when the shared component can support the device view.
- Keep future upstream merges easy: no new dependencies, broad formatting, or
  unrelated changes. Preserve licenses and keep docs concise.
- Never fabricate unavailable metrics. Label unknown or partial costs honestly;
  preserve token totals and pricing semantics.
- Run relevant tests and build both UI variants and native binaries. Compare and
  inspect browser screenshots (light/dark and mobile), and review the final diff
  before deployment. Report checks that could not be verified.
- Before production changes, confirm the host and back up affected files with
  permissions preserved. Record backup paths and exact rollback steps; provide a
  restore script when needed. Keep secret backups outside Git with restricted
  permissions. Ask before downtime or access changes unless explicitly authorized.
