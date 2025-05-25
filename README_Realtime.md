# Go Dispatch Proxy Enhanced - Real-time Monitoring Features

## 🚀 Neue Echtzeit-Features

Diese erweiterte Version des Go Dispatch Proxy bietet umfassende Echtzeit-Monitoring-Funktionen für professionelle Netzwerk-Load-Balancing-Anwendungen.

## 📊 Echtzeit-Verbindungsüberwachung

### Aktive Verbindungen
- **Live-Tracking**: Alle aktiven Verbindungen werden in Echtzeit verfolgt
- **Detaillierte Informationen**: Source IP, Destination IP, Load Balancer, Dauer, Traffic
- **Performance-Limit**: Maximal 500 gleichzeitige Verbindungen für optimale Performance
- **Automatische Bereinigung**: Alte Verbindungen werden automatisch nach 5 Minuten entfernt

### Verbindungsfilterung
```javascript
// Filter nach Source IP
filterConnections("192.168.1.100")

// Filter nach Destination
filterConnections("", "google.com")
```

## 🌐 Web GUI Echtzeit-Dashboard

### Traffic-Statistiken
- **Bytes/Sekunde**: Live-Übertragungsrate
- **Gesamtdatenvolumen**: Kumulierte Datenübertragung
- **Aktive Verbindungen**: Anzahl der aktuell aktiven Verbindungen
- **Verbindungen/Minute**: Durchschnittliche Verbindungsrate

### Load Balancer Traffic-Balken
- **Visuelle Darstellung**: Animierte Balken zeigen Traffic-Verteilung
- **Echtzeit-Updates**: Balken aktualisieren sich alle 2 Sekunden
- **Proportionale Anzeige**: Balkenbreite entspricht dem relativen Traffic

### Verbindungstabelle
- **Filterbare Liste**: Nach Source IP und Destination filterbar
- **Live-Updates**: Automatische Aktualisierung alle 2 Sekunden
- **Individuelle Gewichtung**: Direkte Gewichtungseinstellung pro Verbindung
- **Status-Anzeige**: Active, Closing, Closed Status

## ⚙️ API-Endpunkte

### `/api/connections`
```bash
# Alle aktiven Verbindungen abrufen
curl http://localhost:8888/api/connections

# Mit Filtern
curl "http://localhost:8888/api/connections?source=192.168.1&limit=100"
```

### `/api/traffic`
```bash
# Echtzeit-Traffic-Statistiken
curl http://localhost:8888/api/traffic
```

### `/api/connection/weight`
```bash
# Individuelle Verbindungsgewichtung setzen
curl -X POST http://localhost:8888/api/connection/weight \
  -H "Content-Type: application/json" \
  -d '{
    "source_ip": "192.168.1.100",
    "lb_address": "192.168.1.101:8080",
    "contention_ratio": 5,
    "description": "VIP Client"
  }'
```

## 🔧 Konfiguration

### Umgebungsvariablen
```bash
export WEB_USERNAME="admin"
export WEB_PASSWORD="secure_password"
```

### Erweiterte Konfiguration
```json
{
  "192.168.1.100:8080": {
    "192.168.1.10": {
      "source_ip": "192.168.1.10",
      "contention_ratio": 5,
      "description": "High priority VIP client"
    },
    "10.0.0.0/24": {
      "source_ip": "10.0.0.0/24",
      "contention_ratio": 2,
      "description": "Internal network traffic"
    }
  }
}
```

## 🚀 Verwendung

### Basis-Setup mit Web GUI
```bash
# Mit Web GUI auf Port 8888
./go-dispatch-proxy-enhanced -web 8888 192.168.1.100:8080 192.168.1.101:8080

# Mit benutzerdefinierter Konfiguration
./go-dispatch-proxy-enhanced -web 8888 -config realtime_config.json 192.168.1.100:8080
```

### Tunnel-Modus mit Monitoring
```bash
# SSH-Tunnel mit Echtzeit-Monitoring
./go-dispatch-proxy-enhanced -tunnel -web 8888 user@server1:22 user@server2:22
```

## 📈 Performance-Optimierungen

### Verbindungs-Limits
- **Max. aktive Verbindungen**: 500 (konfigurierbar)
- **Cleanup-Intervall**: 5 Minuten
- **Buffer-Größe**: 32KB für optimale Übertragung

### Memory Management
- **Circular Buffer**: Verhindert Memory-Leaks bei vielen Verbindungen
- **Atomic Counters**: Thread-sichere Statistiken
- **Lazy Cleanup**: Nur bei Bedarf ausgeführt

### Web GUI Performance
- **Update-Intervall**: 2 Sekunden für Traffic, 5 Sekunden für Dashboard
- **Lazy Loading**: Nur sichtbare Daten werden aktualisiert
- **Client-side Filtering**: Reduziert Server-Last

## 🔒 Sicherheit

### Authentifizierung
- **Session-basiert**: 24-Stunden-Sessions
- **Umgebungsvariablen**: Sichere Credential-Verwaltung
- **CSRF-Schutz**: Integriert in alle API-Calls

### Datenschutz
- **Lokale Verarbeitung**: Keine externen Services
- **Memory-only**: Verbindungsdaten nur im RAM
- **Configurable Logging**: Anpassbare Log-Level

## 🛠️ Erweiterte Features

### SQLite Integration (Optional)
```bash
# Mit SQLite für persistente Statistiken
./go-dispatch-proxy-enhanced -web 8888 -sqlite stats.db 192.168.1.100:8080
```

### Monitoring Integration
```bash
# Prometheus Metrics Export
curl http://localhost:8888/metrics

# JSON Stats für externe Tools
curl http://localhost:8888/api/stats
```

## 📱 Mobile Responsive Design

Das Web GUI ist vollständig responsive und funktioniert optimal auf:
- **Desktop**: Vollständige Feature-Set
- **Tablet**: Optimierte Touch-Bedienung
- **Mobile**: Kompakte Ansicht mit allen Funktionen

## 🔧 Troubleshooting

### Häufige Probleme

#### Web GUI lädt nicht
```bash
# Prüfen ob Port verfügbar ist
lsof -i :8888

# Firewall-Einstellungen prüfen
sudo ufw status
```

#### Hoher Memory-Verbrauch
```bash
# Verbindungslimit reduzieren
# In main.go: max_connections = 100
```

#### Performance-Probleme
```bash
# Update-Intervall erhöhen
# In JavaScript: setInterval(..., 5000) // 5 Sekunden
```

## 📊 Monitoring-Metriken

### Verfügbare Metriken
- **Verbindungsanzahl**: Total, Aktiv, Erfolgreich, Fehlgeschlagen
- **Traffic-Volumen**: Bytes In/Out, Gesamtdatenübertragung
- **Performance**: Verbindungen/Minute, Bytes/Sekunde
- **Load Balancer**: Individuelle Statistiken pro LB
- **Source IP**: Verbindungsverteilung nach Quell-IP

### Export-Formate
- **JSON**: Für API-Integration
- **HTML**: Für Web-Dashboard
- **Logs**: Für externe Monitoring-Tools

## 🎯 Use Cases

### Enterprise Load Balancing
- **Multi-WAN**: Mehrere Internet-Verbindungen kombinieren
- **Failover**: Automatisches Umschalten bei Ausfällen
- **QoS**: Priorisierung nach Source IP

### Development & Testing
- **Traffic-Simulation**: Realistische Last-Tests
- **Network-Debugging**: Detaillierte Verbindungsanalyse
- **Performance-Tuning**: Optimierung der Load-Balancing-Algorithmen

### Production Monitoring
- **24/7 Überwachung**: Kontinuierliches Monitoring
- **Alerting**: Integration mit externen Monitoring-Systemen
- **Capacity Planning**: Datenbasierte Infrastruktur-Planung

## 🔄 Updates & Wartung

### Automatische Updates
```bash
# Konfiguration neu laden (ohne Neustart)
curl -X POST http://localhost:8888/api/config/reload

# Statistiken zurücksetzen
curl -X POST http://localhost:8888/api/stats/reset
```

### Backup & Restore
```bash
# Konfiguration sichern
curl http://localhost:8888/api/config > backup.json

# Konfiguration wiederherstellen
curl -X POST http://localhost:8888/api/config -d @backup.json
```

---

**Hinweis**: Diese erweiterte Version ist vollständig rückwärtskompatibel mit der ursprünglichen go-dispatch-proxy Implementierung. Alle bestehenden Konfigurationen und Verwendungsweisen funktionieren weiterhin ohne Änderungen. 