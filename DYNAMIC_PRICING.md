# Dynamic Pricing Configuration

The cost exporter now supports **dynamic pricing** from multiple sources, allowing you to use real-world electricity prices and CO₂ emissions data instead of hardcoded values.

## Pricing Sources (Priority Order)

The cost exporter tries pricing sources in this order:

1. **Custom Configuration File** (highest priority)
2. **Electricity Maps API** (for real-time CO₂ data)
3. **Fallback Static Defaults** (if all else fails)

## Method 1: Custom Pricing Configuration File (Recommended)

Edit `pricing_config.json` with your actual regional electricity prices:

```json
{
  "electricity_prices": {
    "us-east-1": 0.12,
    "eu-west-1": 0.22,
    "default": 0.12
  },
  "co2_intensity": {
    "us-east-1": 390,
    "eu-west-1": 250,
    "default": 400
  }
}
```

The file is automatically loaded when mounted in docker-compose.yml:

```yaml
volumes:
  - ./pricing_config.json:/app/pricing_config.json:ro
environment:
  - PRICING_CONFIG=/app/pricing_config.json
```

### Getting Actual Electricity Prices

**US Regions:**
- Visit [EIA Electricity Data](https://www.eia.gov/electricity/data.php)
- Check commercial/industrial rates for your state
- Convert cents/kWh to dollars/kWh

**EU Regions:**
- Visit [Eurostat Energy Prices](https://ec.europa.eu/eurostat/databrowser/view/nrg_pc_205/default/table)
- Get commercial electricity prices for your country

**Other Regions:**
- Check your local utility company's commercial rates
- Or use national energy statistics websites

## Method 2: Electricity Maps API (Real-Time CO₂)

For real-time grid carbon intensity data:

1. Get an API token from [Electricity Maps](https://www.electricitymaps.com)
2. Set the token in docker-compose.yml:

```yaml
environment:
  - ELECTRICITY_MAPS_TOKEN=your_token_here
```

This provides live CO₂ intensity based on actual grid mix (solar, wind, coal, etc.).

## Method 3: Override Individual Prices

You can override specific prices via environment variables:

```yaml
environment:
  - ENERGY_COST_PER_KWH=0.15  # Override with specific value
  - REGION=us-west-1          # Region affects CO₂ calculations
```

Setting `ENERGY_COST_PER_KWH` to a non-zero value overrides all dynamic pricing.

## Supported Regions

| Region Code | Location | Default Price | Default CO₂ |
|-------------|----------|---------------|-------------|
| us-east-1 | Virginia | $0.12/kWh | 390 gCO₂/kWh |
| us-east-2 | Ohio | $0.11/kWh | 450 gCO₂/kWh |
| us-west-1 | California | $0.19/kWh | 220 gCO₂/kWh |
| us-west-2 | Oregon | $0.10/kWh | 180 gCO₂/kWh |
| eu-west-1 | Ireland | $0.22/kWh | 250 gCO₂/kWh |
| eu-central-1 | Germany | $0.28/kWh | 380 gCO₂/kWh |
| ap-northeast-1 | Tokyo | $0.24/kWh | 480 gCO₂/kWh |
| ap-southeast-1 | Singapore | $0.18/kWh | 520 gCO₂/kWh |
| ap-southeast-2 | Sydney | $0.21/kWh | 650 gCO₂/kWh |

## Caching

Dynamic pricing data is cached for 1 hour to avoid excessive API calls. The cache is automatically refreshed when expired.

## Example Usage

### Scenario 1: On-Premise Datacenter

Create `pricing_config.json` with your actual utility rates:

```json
{
  "electricity_prices": {
    "default": 0.085
  },
  "co2_intensity": {
    "default": 420
  }
}
```

### Scenario 2: Multi-Region Cloud Deployment

Configure all your cloud regions:

```json
{
  "electricity_prices": {
    "us-east-1": 0.12,
    "eu-west-1": 0.22,
    "ap-southeast-1": 0.18
  },
  "co2_intensity": {
    "us-east-1": 390,
    "eu-west-1": 250,
    "ap-southeast-1": 520
  }
}
```

Change `REGION` in docker-compose.yml to match your deployment.

### Scenario 3: Real-Time CO₂ Tracking

Use Electricity Maps for live CO₂ data:

```bash
export ELECTRICITY_MAPS_TOKEN="your_token"
docker-compose up -d --build
```

## Verification

Check the cost exporter logs to see which pricing source is being used:

```bash
docker-compose logs cost-exporter | grep -i pricing
```

You should see messages like:
- `Using dynamic electricity price for us-east-1: $0.12/kWh`
- `Using custom electricity price for us-east-1: $0.08/kWh`
- `Fetched CO₂ intensity for US-VA: 380 gCO₂/kWh`

## Benefits of Dynamic Pricing

✅ **Accurate Cost Projections**: Use real utility rates, not estimates
✅ **Regional Flexibility**: Different prices for different deployments
✅ **Real-Time CO₂**: Track environmental impact with live grid data
✅ **Easy Updates**: Change pricing without rebuilding containers
✅ **API Integration**: Automate pricing updates via APIs

## Troubleshooting

**Problem**: Prices seem wrong
- Check `pricing_config.json` is mounted correctly
- Verify JSON syntax is valid
- Check docker logs for loading errors

**Problem**: API not working
- Verify API token is set correctly
- Check internet connectivity from container
- Ensure API quota isn't exceeded

**Problem**: Using fallback prices
- This is normal if no config file or API is configured
- Fallback prices are reasonable defaults for most regions
- Add custom config for accurate pricing
