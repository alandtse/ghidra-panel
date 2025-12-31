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
