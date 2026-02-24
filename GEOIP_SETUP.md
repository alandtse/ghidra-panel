# IP Geolocation Setup (Optional)

Display flag emoji + city/country for IP addresses in audit logs.

## Setup

**1. Get GeoLite2 database** (free, requires signup):
   - Sign up: https://www.maxmind.com/en/geolite2/signup
   - Download: GeoLite2-City.mmdb (binary format)

**2. Place database file:**
Place the file in your project root or a persistent data directory.

**3. Configure panel:**
Update your `config.yaml` with the path to the database file. If using Docker, ensure the file is mounted into the container (e.g., at `/data/GeoLite2-City.mmdb`).

```yaml
# config.yaml (example for Docker)
geoip_database: "/data/GeoLite2-City.mmdb"
```

**4. Restart panel**
If using Docker: `docker compose restart panel`

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

MaxMind updates monthly. Download new `.mmdb` file and replace existing one.

### Auto-update with Docker (Recommended)

The `docker-compose.yml` includes a sidecar service that automatically manages your database. To enable it:

1. Add your MaxMind credentials to `config.yaml`:
   ```yaml
   # config.yaml
   maxmind_account_id: "your_account_id"
   maxmind_license_key: "your_license_key"
   ```
2. Restart your stack:
   ```bash
   docker compose up -d
   ```

That's it! The sidecar will download the database to the correct location (`/data/GeoLite2-City.mmdb`) and the panel will automatically find and use it.

### Manual update

Download the new `.mmdb` file and replace the existing one in your data directory.

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
