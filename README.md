# CPA Device Usage Keeper

Track coding-agent usage across your devices in a native CPA plugin. Uses **ccusage** for collection and Keeper's dashboard and pricing settings for estimated costs.

Independent fork of [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper), based on stable **v2.1.0**. Not the official project.

## Install on your devices

**Public release pending: these commands will work only after the bootstrap scripts and client bundles are published.** Private bundles are available for [controlled testing](docs/cpa-testing.md).

### Linux and macOS

```sh
curl -fsSL https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/latest/download/bootstrap.sh | sh
```

### Windows — PowerShell

```powershell
curl.exe -fsSL https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/latest/download/bootstrap.ps1 -o bootstrap.ps1
# Inspect the downloaded script, then run:
.\bootstrap.ps1
```

Enroll a device in the dashboard, then enter its upload token and your dashboard's HTTPS URL when prompted. Review the discovered paths: **Enter accepts all checked paths; numbers toggle selections.** Approved usage uploads every 15 minutes while the user scheduler is running.

Installers default to the latest stable release and ask before changing an installed version. **Windows in-place upgrades are not yet supported.** Supported: Linux x86-64, macOS Intel/Apple Silicon, Windows x64/ARM64. Unix requires curl, Python 3, and Bash. See [installation details and requirements](docs/device-install.md).

## Server and details

The native plugin hosts the dashboard and backend inside CPA; no separate service stack is needed. Configure a TLS reverse proxy to its listener. See [CPA setup](docs/cpa-testing.md) and [native configuration](docs/native/README.md).

Uploads contain usage aggregates, not conversations or provider credentials. Request counts, latency, and success rates are unavailable from daily device history. This project remains under controlled testing; native macOS/Windows validation and production load/recovery checks are incomplete.

- [Source selection and custom paths](docs/device-sources.md)
- [Platform support](docs/device-platforms.md)
- [Project scope](DEVICE-USAGE.md) · [Original upstream README](docs/upstream/README.md)

## License

[MIT](LICENSE). Original CPA Usage Keeper notices are retained.
