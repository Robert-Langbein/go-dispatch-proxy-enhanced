#!/bin/sh
# Entrypoint für den Dispatch Proxy auf UNRAID System zur automatischen IP-Adressen Ermittlung und Verwendung

# --- Web-Credentials konfigurieren ----------------------------------
WEB_USERNAME="${WEB_USERNAME:-admin}"
WEB_PASSWORD="${WEB_PASSWORD:-admin}"
export WEB_USERNAME
export WEB_PASSWORD

# --- aktuelle IPv4-Adressen ermitteln ------------------------------
IP_BR0=$(ip -4 -o addr show br0  | awk '{print $4}' | cut -d/ -f1)
IP_BR1=$(ip -4 -o addr show br1  | awk '{print $4}' | cut -d/ -f1)
# Fallback, falls Interface nicht existiert
[ -z "$IP_BR0" ] && { echo "br0 hat keine IPv4-Adresse"; exit 1; }
[ -z "$IP_BR1" ] && { echo "br1 hat keine IPv4-Adresse"; exit 1; }

# --- Proxy starten --------------------------------------------------
echo "LAUNCHING DISPATCH PROXY"
echo "IP BR0: ${IP_BR0}@1"
echo "IP BR1: ${IP_BR1}@1"
echo "Web Username: ${WEB_USERNAME}"
echo "Web Password: [***]"
exec go-dispatch-proxy \
     -web 8090 \
     -lhost 0.0.0.0 \
     -lport 33333 \
     "${IP_BR0}@1" \
     "${IP_BR1}@1"