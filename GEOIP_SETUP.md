# IP Geolocation Setup (Optional)

Display flag emoji + city/country for IP addresses in audit logs.

## Setup

**1. Get MaxMind credentials** (free, requires signup):
   - Sign up: https://www.maxmind.com/en/geolite2/signup
   - Go to your account dashboard and generate a License Key
   - Note down your Account ID and the License Key

**2. Configure panel:**
Update your `config.yaml` with your credentials. The Docker stack will automatically download and update the database.

```yaml
# config.yaml (example for Docker)
maxmind_account_id: "your_account_id"
maxmind_license_key: "your_license_key"
```

**3. Restart stack**
If using Docker: `docker compose up -d`

## Verification

**With GeoIP:**
```
IP Address    Location
[::1]         🏠 localhost
203.0.113.45  🇺🇸 San Francisco, United States
198.51.100.78 🇬🇧 London, United Kingdom
```

**Without GeoIP:**
```
IP Address    Location
[::1]         -
203.0.113.45  -
```

Check logs: `GeoIP database loaded: GeoLite2-City.mmdb`

## Updates

The included `geoipupdate` docker service automatically checks for constraints and downloads new updates weekly. You don't need to manually update anything!

### Manual or Non-Docker setup

If you are not using Docker, you can manually download the GeoLite2-City.mmdb file and place it in the same directory as the executable, or explicitly set the `--geoip_database` CLI flag or YAML configuration.

## Troubleshooting

**Locations show "-":**
- Verify file path in config
- Check file permissions
- Look for "GeoIP database loaded" in logs

**Wrong location:**
- GeoLite2 accuracy: ~95% country, ~80% city
- VPNs show VPN server location (expected)
- Private IPs (192.168.x.x) have no location data

## Details

- **File size:** ~60 MB
- **Lookup time:** ~1ms per IP
- **Privacy:** Lookups are local, no external API calls
- **Storage:** Location not stored in DB, computed on-demand
