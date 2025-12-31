# IP Geolocation Setup (Optional)

Display flag emoji + city/country for IP addresses in audit logs.

## Setup

**1. Get GeoLite2 database** (free, requires signup):
   - Sign up: https://www.maxmind.com/en/geolite2/signup
   - Download: GeoLite2-City.mmdb (binary format)

**2. Place database file:**
```
E:\path\to\ghidra-panel\GeoLite2-City.mmdb
```

**3. Configure panel:**
```yaml
# config.yaml
geoip_database: "GeoLite2-City.mmdb"
```

**4. Restart panel**

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

**Auto-update (optional):** Use [geoipupdate](https://github.com/maxmind/geoipupdate) tool.

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
