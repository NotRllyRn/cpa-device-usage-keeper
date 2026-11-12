# Device model pricing

Device costs use Keeper's validation and token-cost arithmetic with exclusive
ccusage token categories adapted to Keeper's inclusive-input convention.
They are API-equivalent estimates, not subscription invoices.

Settings reuses Keeper's editor. Add selects an uploaded, unconfigured model;
Edit changes an existing configuration. Additional model identifiers can be added
to the choices manually. Saving refreshes historical estimates automatically.
There is no request-recalculation job for daily aggregates.

Sync Prices opens a device adapter using Keeper's existing LiteLLM and Models.dev
catalog readers. Select the uploaded identifier, fetch a source, search and select
a source model, then review and save in Keeper's editor. This does not automatically
map aliases or overwrite all configurations. The saved configuration uses the exact
uploaded identifier. Missing cache rates require explicit operator input; missing
input/output rates must be entered manually. Zero means explicitly free, not unknown.
Provider tiers, long-context rates and request-specific conditions are not inferred.

Catalog access is authenticated and limited to the two built-in upstream URLs.
If a source fails, try the other source or enter verified rates manually. No
collector costs or fabricated model aliases are used as price evidence.

The overview shows model input/output/cache tokens and calculated cost; daily
history gives the day breakdown. Incomplete totals are labeled. An incomplete
zero subtotal is displayed as Unknown rather than misleadingly as free usage.
Request-dependent rules remain unknown because daily reports cannot evaluate them.
