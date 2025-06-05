# Gateway Mode - Load Balancing Default Gateway

Der **Gateway Mode** ist eine revolutionäre Erweiterung des go-dispatch-proxy, die es ermöglicht, den Proxy als Standard-Gateway zu verwenden und dabei Load Balancing für den gesamten Netzwerkverkehr zu bieten.

## 🚀 Überblick

Im Gateway-Modus fungiert der go-dispatch-proxy als:
- **Standard-Gateway** für Client-Geräte
- **Transparenter Proxy** mit Load Balancing
- **DNS-Server** mit Weiterleitung
- **NAT-Router** mit intelligenter Verkehrsverteilung

## 🔧 Funktionsweise

### Architektur
```
[Client] → [Gateway/Proxy] → [Load Balancer 1] → [Internet]
                          → [Load Balancer 2] → [Internet]
                          → [Load Balancer N] → [Internet]
```

### Komponenten
1. **Transparent Proxy**: Fängt TCP-Verkehr ab und leitet ihn über Load Balancer weiter
2. **DNS-Server**: Beantwortet DNS-Anfragen und leitet sie an Upstream-Server weiter
3. **iptables-Integration**: Automatische Konfiguration von Routing-Regeln
4. **NAT-Funktionalität**: Masquerading für ausgehenden Verkehr

## 📋 Verwendung

### Command Line

#### Basis-Gateway-Modus
```bash
sudo ./go-dispatch-proxy -gateway \
  -gateway-ip 192.168.100.1 \
  -subnet 192.168.100.0/24 \
  192.168.1.10@3 10.81.201.18@2
```

#### Erweiterte Konfiguration
```bash
sudo ./go-dispatch-proxy -gateway \
  -gateway-ip 192.168.100.1 \
  -subnet 192.168.100.0/24 \
  -transparent-port 8888 \
  -dns-port 5353 \
  -nat-interface eth0 \
  
  -web 80 \
  192.168.1.10@3 10.81.201.18@2
```

### Parameter

| Parameter | Standard | Beschreibung |
|-----------|----------|--------------|
| `-gateway` | false | Aktiviert Gateway-Modus |
| `-gateway-ip` | 192.168.100.1 | IP-Adresse des Gateways |
| `-subnet` | 192.168.100.0/24 | Subnetz für Clients |
| `-transparent-port` | 8888 | Port für transparenten Proxy |
| `-dns-port` | 5353 | Port für DNS-Server |
| `-nat-interface` | auto | Netzwerk-Interface für NAT |
| `-auto-config` | true | Automatische iptables-Konfiguration |


## 🔐 Voraussetzungen

### System-Anforderungen
- **Linux-System** (für SO_ORIGINAL_DST Support)
- **Root-Berechtigung** (für iptables und transparenten Proxy)
- **iptables** installiert
- **Kernel mit netfilter** Support

### Berechtigungen
```bash
# Root-Berechtigung erforderlich
sudo ./go-dispatch-proxy -gateway ...

# Oder mit Capabilities (empfohlen)
sudo setcap cap_net_admin,cap_net_raw=eip ./go-dispatch-proxy
```

## 🌐 Netzwerk-Setup

### 1. Gateway-Konfiguration

#### Automatische Konfiguration (Standard)
Der Proxy konfiguriert automatisch:
- IP-Forwarding aktivieren
- iptables-Regeln für transparenten Proxy
- NAT-Regeln für ausgehenden Verkehr
- DNS-Weiterleitung

#### Manuelle Konfiguration
```bash
# IP-Forwarding aktivieren
echo 1 > /proc/sys/net/ipv4/ip_forward

# Transparenter Proxy
iptables -t nat -A PREROUTING -s 192.168.100.0/24 -p tcp --dport 1:65535 -j REDIRECT --to-port 8888

# NAT für ausgehenden Verkehr
iptables -t nat -A POSTROUTING -s 192.168.100.0/24 -o eth0 -j MASQUERADE

# DNS-Weiterleitung
iptables -t nat -A PREROUTING -s 192.168.100.0/24 -p udp --dport 53 -j REDIRECT --to-port 5353
```

### 2. Client-Konfiguration

#### Statische IP-Konfiguration
```bash
# Auf Client-Geräten
ip addr add 192.168.100.50/24 dev eth0
ip route add default via 192.168.100.1
echo "nameserver 192.168.100.1" > /etc/resolv.conf
```

#### DHCP-Server (separat erforderlich)
```bash
# Beispiel mit dnsmasq
dnsmasq --interface=br0 \
        --dhcp-range=192.168.100.10,192.168.100.100,12h \
        --dhcp-option=3,192.168.100.1 \
        --dhcp-option=6,192.168.100.1
```

## 🎛️ WebUI-Steuerung

### Gateway-Dashboard
Die WebUI bietet eine intuitive Oberfläche für:
- **Gateway-Status** anzeigen
- **Ein-/Ausschalten** des Gateway-Modus
- **Konfiguration** ändern
- **Verkehrsstatistiken** überwachen
- **iptables-Regeln** einsehen

### API-Endpunkte

#### Gateway-Status abrufen
```bash
curl -X GET http://localhost:80/api/gateway \
  -H "Cookie: session=your_session_id"
```

#### Gateway aktivieren/deaktivieren
```bash
curl -X POST http://localhost:80/api/gateway/toggle \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your_session_id" \
  -d '{"enabled": true}'
```

#### Konfiguration ändern
```bash
curl -X POST http://localhost:80/api/gateway/config \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your_session_id" \
  -d '{
    "gateway_ip": "192.168.100.1",
    "subnet_cidr": "192.168.100.0/24",
    "transparent_port": 8888,
    "dns_port": 5353
  }'
```

## 📊 Load Balancing im Gateway-Modus

### Source IP Awareness
Auch im Gateway-Modus profitieren Sie von der erweiterten Source IP-basierten Load Balancing-Funktionalität:

```json
{
  "192.168.1.10:0": {
    "192.168.100.50": {
      "source_ip": "192.168.100.50",
      "contention_ratio": 5,
      "description": "High-priority client device"
    },
    "192.168.100.0/24": {
      "source_ip": "192.168.100.0/24",
      "contention_ratio": 2,
      "description": "Default for all gateway clients"
    }
  }
}
```

### Verkehrsverteilung
- **Transparente Weiterleitung**: Clients merken nichts vom Load Balancing
- **Session-Persistenz**: Verbindungen bleiben konsistent
- **Failover**: Automatischer Wechsel bei Ausfall eines Load Balancers

## 🔍 Monitoring & Debugging

### Logs überwachen
```bash
# Debug-Modus aktivieren
sudo ./go-dispatch-proxy -gateway -debug ...

# Logs in Echtzeit verfolgen
tail -f /var/log/syslog | grep dispatch-proxy
```

### Verkehrsanalyse
```bash
# Aktive Verbindungen anzeigen
curl http://localhost:80/api/connections

# Traffic-Statistiken
curl http://localhost:80/api/traffic

# Gateway-spezifische Statistiken
curl http://localhost:80/api/gateway
```

### iptables-Regeln prüfen
```bash
# NAT-Tabelle anzeigen
iptables -t nat -L -n -v

# Mangle-Tabelle anzeigen
iptables -t mangle -L -n -v

# Aktive Verbindungen
netstat -tuln | grep :8888
```

## 🚨 Troubleshooting

### Häufige Probleme

#### 1. "Permission denied" Fehler
```bash
# Lösung: Root-Berechtigung erforderlich
sudo ./go-dispatch-proxy -gateway ...
```

#### 2. "Failed to get original destination"
```bash
# Lösung: SO_ORIGINAL_DST Support prüfen
modprobe xt_REDIRECT
```

#### 3. Clients können nicht ins Internet
```bash
# IP-Forwarding prüfen
cat /proc/sys/net/ipv4/ip_forward

# iptables-Regeln prüfen
iptables -t nat -L POSTROUTING -n -v
```

#### 4. DNS funktioniert nicht
```bash
# DNS-Server Status prüfen
netstat -ulpn | grep :5353

# DNS-Weiterleitung testen
dig @192.168.100.1 google.com
```

### Performance-Optimierung

#### Kernel-Parameter
```bash
# TCP-Buffer erhöhen
echo 'net.core.rmem_max = 16777216' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 16777216' >> /etc/sysctl.conf

# Connection Tracking erhöhen
echo 'net.netfilter.nf_conntrack_max = 65536' >> /etc/sysctl.conf
```

#### Proxy-Parameter
```bash
# Mehr Goroutines für hohen Durchsatz
# (wird automatisch basierend auf Systemressourcen angepasst)
```

## 🔧 Erweiterte Konfiguration

### Mehrere Subnetze
```bash
# Mehrere Subnetze unterstützen
iptables -t nat -A PREROUTING -s 192.168.100.0/24 -p tcp --dport 1:65535 -j REDIRECT --to-port 8888
iptables -t nat -A PREROUTING -s 10.0.0.0/24 -p tcp --dport 1:65535 -j REDIRECT --to-port 8888
```

### VLAN-Support
```bash
# VLAN-Interfaces konfigurieren
ip link add link eth0 name eth0.100 type vlan id 100
ip addr add 192.168.100.1/24 dev eth0.100
```

### Hochverfügbarkeit
```bash
# Mehrere Gateway-Instanzen mit keepalived
# (Konfiguration außerhalb des Scopes dieses Dokuments)
```

## 📈 Use Cases

### 1. Home Office Setup
- **Szenario**: Kombination von DSL und LTE für bessere Bandbreite
- **Konfiguration**: Gateway-Modus mit automatischem Failover
- **Vorteil**: Transparente Nutzung beider Verbindungen

### 2. Small Business
- **Szenario**: Load Balancing zwischen mehreren Internet-Providern
- **Konfiguration**: Source IP-basierte Priorisierung für verschiedene Abteilungen
- **Vorteil**: Optimale Bandbreitennutzung und Ausfallsicherheit

### 3. Development Environment
- **Szenario**: Test verschiedener Netzwerkpfade für Anwendungen
- **Konfiguration**: Dynamische Umschaltung über WebUI
- **Vorteil**: Einfache Simulation verschiedener Netzwerkbedingungen

## 🔒 Sicherheitshinweise

### Firewall-Regeln
```bash
# Nur notwendige Ports öffnen
iptables -A INPUT -p tcp --dport 80 -j ACCEPT    # WebUI
iptables -A INPUT -p tcp --dport 8888 -j ACCEPT  # Transparent Proxy
iptables -A INPUT -p udp --dport 5353 -j ACCEPT  # DNS
```

### Access Control
- WebUI mit Authentifizierung schützen
- Starke Passwörter verwenden
- HTTPS für WebUI konfigurieren (empfohlen)

### Monitoring
- Regelmäßige Überprüfung der Logs
- Überwachung ungewöhnlicher Verkehrsmuster
- Backup der iptables-Regeln

## 🚀 Zukunftspläne

### Geplante Features

- **IPv6-Support**: Vollständige IPv6-Unterstützung
- **QoS-Integration**: Traffic Shaping und Priorisierung
- **VPN-Integration**: Nahtlose VPN-Unterstützung
- **Clustering**: Hochverfügbarkeit mit mehreren Gateway-Instanzen

### Roadmap
1. **v4.0**: IPv6-Support und erweiterte Funktionen
2. **v4.1**: IPv6-Support
3. **v4.2**: QoS und Traffic Shaping
4. **v5.0**: Enterprise Features und Clustering

## iptables Backup and Restore

### Overview

The gateway mode now includes enhanced iptables backup and restore functionality to ensure system safety when automatically configuring network rules.

### Key Features

- **Default Safety**: AutoConfigure is now **disabled by default** - users must explicitly enable it
- **Automatic Backup**: When AutoConfigure is enabled, the original iptables rules are automatically backed up before applying gateway rules
- **Manual Restore**: Original iptables rules can be restored at any time via the WebInterface
- **Backup Status**: Real-time status of backup availability and configuration state

### Configuration

The `auto_configure` option is now **disabled by default** for safety:

```bash
# AutoConfigure disabled (default) - manual iptables configuration required
./go-dispatch-proxy-enhanced -gateway -gateway-ip 192.168.100.1 -subnet 192.168.100.0/24

# AutoConfigure enabled - automatic iptables configuration with backup
./go-dispatch-proxy-enhanced -gateway -gateway-ip 192.168.100.1 -subnet 192.168.100.0/24 -auto-config
```

### API Endpoints

#### Get iptables Backup Status
```bash
curl -X GET http://localhost:8090/api/gateway/iptables/backup \
  -H "Content-Type: application/json"
```

Response:
```json
{
  "backup_exists": true,
  "backup_file": "iptables_backup.rules",
  "backup_timestamp": "2024-01-15 14:30:25",
  "backup_size": 2048,
  "backup_file_exists": true,
  "is_configured": true,
  "auto_configure": true
}
```

#### Restore Original iptables Rules
```bash
curl -X POST http://localhost:8090/api/gateway/iptables/restore \
  -H "Content-Type: application/json"
```

Response:
```json
{
  "success": true,
  "message": "Original iptables rules restored successfully"
}
```

### WebInterface Integration

The gateway configuration in the WebInterface now displays:

- **AutoConfigure Status**: Whether automatic iptables configuration is enabled
- **Backup Status**: Whether a backup of original rules exists
- **Backup Timestamp**: When the backup was created
- **Restore Button**: One-click restore of original iptables rules

### Safety Features

1. **No Backup Without AutoConfigure**: Backup is only created when AutoConfigure is explicitly enabled
2. **Single Backup**: Only one backup is created per session to preserve the original state
3. **Automatic Cleanup**: Original rules are restored when gateway mode is disabled (if AutoConfigure was used)
4. **Manual Override**: Users can restore original rules at any time via WebInterface

### Backup File Location

- **Default Location**: `iptables_backup.rules` in the application directory
- **Format**: Standard iptables-save format
- **Permissions**: 644 (readable by owner and group)

### Error Handling

- If backup creation fails, gateway mode continues but without automatic rule configuration
- If restore fails, detailed error messages are provided via API and logs
- Missing backup files are detected and reported appropriately

### Migration Notes

**Important**: Existing installations will have AutoConfigure **disabled by default** after upgrade. Users who want automatic iptables configuration must explicitly enable it in the settings or via command line flag.

---

**Hinweis**: Der Gateway-Modus ist eine experimentelle Funktion und sollte in Produktionsumgebungen mit Vorsicht eingesetzt werden. Testen Sie die Konfiguration gründlich in einer isolierten Umgebung, bevor Sie sie in kritischen Netzwerken einsetzen. 