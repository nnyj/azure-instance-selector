# azure-instance-selector

<div align="center">

[![Stars](https://img.shields.io/github/stars/nnyj/azure-instance-selector?style=for-the-badge&labelColor=555&color=e3b341)](https://github.com/nnyj/azure-instance-selector/stargazers)
[![Downloads](https://img.shields.io/github/downloads/nnyj/azure-instance-selector/total?style=for-the-badge&labelColor=555&color=2ea44f)](https://github.com/nnyj/azure-instance-selector/releases)
[![Latest Release](https://img.shields.io/github/v/release/nnyj/azure-instance-selector?style=for-the-badge&label=Latest%20Release&labelColor=555&color=3572d6)](https://github.com/nnyj/azure-instance-selector/releases/latest)
[![Build](https://img.shields.io/github/actions/workflow/status/nnyj/azure-instance-selector/release.yml?style=for-the-badge&labelColor=555)](https://github.com/nnyj/azure-instance-selector/actions)

</div>

Filter Azure VM sizes by vCPU, memory, GPU, architecture, and price, with on-demand and spot prices in one command, no Azure account required.

## Features

- Filter by vCPU count, memory, GPU count, CPU architecture (x64/arm64), family regex, name regex
- On-demand and spot prices per region, with spot savings percentage
- Filter and sort by spot or on-demand price
- No credentials needed: capabilities from Vantage public catalog, prices from Azure Retail Prices API
- Optional authenticated ARM source for subscription-scoped availability and restrictions
- Output modes: table, wide table, one-line (scripting), JSON
- Local cache with 7-day TTL, works offline after first fetch

## Install

```sh
go install github.com/nnyj/azure-instance-selector/cmd@latest
```

Or download a binary from [releases](https://github.com/nnyj/azure-instance-selector/releases), or build from source:

```sh
git clone https://github.com/nnyj/azure-instance-selector
cd azure-instance-selector
go build -o azure-instance-selector ./cmd
```

## Usage

```sh
azure-instance-selector --vcpus-min 4 --vcpus-max 8 --usage-class spot
azure-instance-selector -a arm64 --memory-min 16gb --price-per-hour-max 0.10
azure-instance-selector --gpus-min 1 --spot-capable -o table-wide
azure-instance-selector --vcpus 4 --memory 16gb -o one-line
```

Filter flags `--vcpus`, `--memory`, `--gpus`, `--price-per-hour` each accept `-min`/`-max` variants. Memory accepts suffixed sizes (`16gb`, `512mb`). Exact flag sets min = max.

| Flag                           | Default                      | Description                                                        |
| ------------------------------ | ---------------------------- | ------------------------------------------------------------------ |
| `-a, --cpu-architecture`       |                              | `x64`, `arm64` (aliases `x86_64`, `amd64`)                         |
| `--usage-class`                | `on-demand`                  | `spot` sorts and price-filters by spot price                       |
| `--os`                         | `linux`                      | `linux` or `windows` pricing                                       |
| `--spot-capable`               |                              | only SKUs with the spot capability flag                            |
| `--accelerated-networking`     |                              | require accelerated networking                                     |
| `--premium-io`                 |                              | require premium storage support                                    |
| `--family`                     |                              | family regex, e.g. `^dsv5$`                                        |
| `--allow-list` / `--deny-list` |                              | keep/exclude by name regex                                         |
| `-r, --region`                 | `eastus`                     | Azure region (armRegionName)                                       |
| `-o, --output`                 | `table`                      | `table`, `table-wide`, `one-line`, `json`                          |
| `--max-results`                | `20`                         | result cap, `0` = all                                              |
| `--sku-source`                 | `vantage`                    | `vantage` (anonymous) or `arm` (authed)                            |
| `--subscription-id`            |                              | for `arm` source, also reads `AZURE_SUBSCRIPTION_ID` or `az login` |
| `--cache-dir`                  | `~/.azure-instance-selector` | cache location                                                     |
| `--cache-ttl`                  | `168h`                       | cache freshness window                                             |
| `--refresh`                    |                              | force re-fetch of both catalogs                                    |

## How it works

Two anonymous data sources are joined by ARM SKU name. Capabilities (vCPU, memory, GPU, arch, region availability) come from [Vantage's public catalog](https://instances.vantage.sh/azure), rebuilt daily from the ARM Resource SKUs API. Prices come from the [Azure Retail Prices API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices), a public unauthenticated endpoint that includes spot meters. Both are cached locally, so repeated queries hit no network.

`--sku-source arm` queries the ARM Resource SKUs API directly with your subscription, adding subscription-specific truth: quota restrictions, zone availability, and sizes hidden from your subscription type. Auth resolves via the standard Azure credential chain (`az login`, env vars, managed identity).

> [!NOTE]
> Vantage arch data is missing for ~180 legacy sizes (assumed x64). Spot prices are shown for any SKU with a spot meter, including some (B-series) whose real spot eligibility is doubtful. Use `--spot-capable` to filter to deployable spot.

## Credits

- [amazon-ec2-instance-selector](https://github.com/aws/amazon-ec2-instance-selector): original project this tool was ported from

## License

[MIT](LICENSE)
