# Administrator Operations Guide

This repository includes a full containerized setup for deploying both the SREPanel and a customized Ghidra Server instances anywhere using Docker Compose.

## Prerequisites
- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)

## Quick Start

1. **Configure SREPanel**
   Create a `config.yaml` file based on the example provided:
   ```bash
   cp config.example.yaml config.yaml
   ```
   Edit the file and ensure that `ghidra.grpc_addr` points to the internal Docker service. Since they are on the same Docker network, it should be:
   ```yaml
   ghidra:
     grpc_addr: "ghidra:13103"
   ```

2. **Start the Services**
   Run the following command in the root of the repository:
   ```bash
   docker compose up -d --build
   ```

   *Note: The first time you run this, it will download the Ghidra release zip, compile the `jaas` Gradle plugin, build the Go backend, and patch the server configurations. This may take a few minutes.*

## Advanced Configuration (Environment Variables)

Alternatively, you can skip configuring a static `config.yaml` file by passing environment variables directly in your `docker-compose.yml` or through a `.env` file.

Prefix any configuration key with `SRE_`. For example:

```yaml
  panel:
    environment:
      - SRE_BASE_URL=https://panel.example.com
      - SRE_COMMUNITY_NAME=My Community
      - SRE_GEOIP_DATABASE=/data/GeoLite2-City.mmdb
      # Nested OAuth Map overrides
      - SRE_OAUTH_GITHUB_CLIENT_ID=abc...
```
*Note: Make sure to still define the `panel_data` mount so the SQLite database persists!*

## Volume Management

The `docker-compose.yml` mounts two persistent volumes:

- `panel_data`: Stores the `/data` directory shared by both containers. It contains `ghidra_panel.db` (the SQLite database), ensuring that both the web panel and Ghidra JAAS plugin read from the exact same file.
- `ghidra_repos`: Stores the `/repos` directory where Ghidra server actually saves its project files.

Your local `./config.yaml` is bind-mounted directly into the container as `/data/config.yaml`.

## Remote Connectivity (RMI Hostname)

If you are deploying this to a remote server (like Proxmox, AWS, or DigitalOcean) and plan to connect your local Ghidra GUI client to it, you **must override the `GHIDRA_PUBLIC_HOSTNAME`**.

By default, the Ghidra Server advertises its connection IP to clients as `127.0.0.1`. This works perfectly if you are deploying and testing on your local machine. However, if deployed remotely, your local GUI will falsely try to route traffic to its own `127.0.0.1` and fail to connect.

To fix this, update `docker-compose.yml` to set your server's public IP or Domain Name:

```yaml
services:
  ghidra:
    environment:
      - GHIDRA_PUBLIC_HOSTNAME=your-server-ip.com
```

## Server Maintenance

### Complete Volume Migration (Moving Servers)
The SQLite database and the Ghidra repositories are stored in named volumes (`panel_data` and `ghidra_repos`). To migrate to a new server, you can create a tarball of both volumes:

1. Stop the containers:
   ```bash
   docker compose down
   ```
2. Backup the volumes using a temporary Alpine container:
   ```bash
   # Backup the Panel Database & Configs
   docker run --rm -v ghidra-panel_panel_data:/data -v $(pwd):/backup alpine tar -czvf /backup/panel_data_backup.tar.gz -C /data .

   # Backup the Ghidra Repos
   docker run --rm -v ghidra-panel_ghidra_repos:/repos -v $(pwd):/backup alpine tar -czvf /backup/ghidra_repos_backup.tar.gz -C /repos .
   ```
3. Transfer the `.tar.gz` files to the new server. Create the empty volumes and extract the data:
   ```bash
   docker volume create ghidra-panel_panel_data
   docker volume create ghidra-panel_ghidra_repos

   docker run --rm -v ghidra-panel_panel_data:/data -v $(pwd):/backup alpine sh -c "cd /data && tar -xzvf /backup/panel_data_backup.tar.gz"
   docker run --rm -v ghidra-panel_ghidra_repos:/repos -v $(pwd):/backup alpine sh -c "cd /repos && tar -xzvf /backup/ghidra_repos_backup.tar.gz"
   ```

### Proxmox / Unraid / Advanced Hypervisors
If you are running the Docker host inside a hypervisor that supports full VM/LXC snapshots (like Proxmox or Unraid), it is often better to use **Bind Mounts**. This stores your data in a transparent directory on the host rather than hidden in Docker's internal volumetric storage.

To do this, modify the `volumes:` section of both services in `docker-compose.yml` to point to a local directory:
```yaml
    volumes:
      - ./data/panel:/data    # SQLite DB and config
      - ./data/repos:/repos   # Ghidra repos
```
*Note: Make sure these directories are owned by the correct user if you hit permission issues.*

## Customizing Ghidra Versions

By default, the `docker-compose.yml` configures the build to download **Ghidra 12.0.3 PUBLIC**.
If you want to deploy a different version, you edit the `GHIDRA_ZIP_URL` argument directly in `docker-compose.yml`:

```yaml
services:
  ghidra:
    build:
      context: .
      args:
        GHIDRA_ZIP_URL: "https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_12.0.3_build/ghidra_12.0.3_PUBLIC_20260210.zip"
```

*Note: You must ensure that `jaas/build.gradle` defines the correct `protobufVersion` and `java.toolchain` matching your target Ghidra release before building.*

## Updating

To pull in new code changes from the repository and restart the services, simply run:
```bash
git pull
docker compose up -d --build
```
This forces a clean recompilation of the plugin and Go binary.


---

# Ghidra Server Setup

Install the JAAS plugin on your Ghidra Server to enable panel authentication.

## Prerequisites

- Ghidra Server installed
- Java JDK (same version as Ghidra) - check with `java -version`
- Panel SQLite database accessible

## Installation

### Quick Install

```bash
cd jaas
make build install  # Requires GHIDRA_SERVER_DIR env var
```

### Manual Install

**1. Build JAR:**
```bash
cd jaas
./gradlew build  # Output: build/libs/ghidra-panel-jaas.jar
```

**2. Create `ghidra/server/jaas.conf`:**
```java
auth {
    re.mkw.srejaas.PanelLoginModule REQUIRED
        JDBC="jdbc:sqlite:/path/to/panel.db"
    ;
};
```

**3. Edit `ghidra/server/server.conf`:**
```properties
# Main class
wrapper.java.app.mainclass=re.mkw.srejaas.PanelServer

# Classpath
wrapper.java.classpath.Panel=/path/to/ghidra-panel-jaas.jar

# JAAS config
wrapper.app.parameter.1=-a4
wrapper.app.parameter.2=/path/to/jaas.conf

# gRPC port (default: 13103)
wrapper.app.parameter.gRPC=-g13103

# Java 25 compatibility (if needed)
wrapper.java.additional.10=--add-opens=java.base/java.lang.invoke=ALL-UNNAMED
```

**4. Start Ghidra Server:**
```bash
./svrAdmin
```

Panel should show "● Ghidra Server Online"

## Java 25 Compatibility

If using Java 25, add to `server.conf`:
```properties
wrapper.java.additional.10=--add-opens=java.base/java.lang.invoke=ALL-UNNAMED
```

## Troubleshooting

**Panel shows "Offline":**
- Check Ghidra Server logs: `ghidra/server/logs/wrapper.log`
- Verify gRPC port not in use: `netstat -an | grep 13103`
- Test database path in `jaas.conf`

**"ClassNotFoundException":**
- Verify JAR path in `server.conf` `wrapper.java.classpath.Panel`
- Ensure absolute path, not relative

**"SQLException: database locked":**
- Stop all Ghidra Servers accessing same database
- Check file permissions

**Port conflict:**
- Change gRPC port: `wrapper.app.parameter.gRPC=-g13104`
- Update panel config: `ghidra.grpc_addr: "localhost:13104"`

## Development

**Hot reload plugin:**
```bash
cd jaas && make build
# Restart Ghidra Server to load new JAR
```

**Enable debug logging:**
```properties
# server.conf
wrapper.console.loglevel=DEBUG
```


---

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
