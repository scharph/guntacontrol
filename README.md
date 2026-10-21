# guntacontrol

Lokale Überwachung und Steuerung einer **Guntamatic Biostar Flex** über deren
offene HTTP-Schnittstelle – ein kleiner Go-Server mit eingebettetem Angular-Web-UI,
Prometheus-Metriken, SQLite-Verlauf, Loxone-Anbindung und Audit-Log.

## Features

- **Dashboard** (Angular 22, zoneless/signals) – Live-Werte, Betriebsmodus und
  Heizkreis-Programme schalten, mit Schreibschutz.
- **Zeiten** – Anzeige der Uhrenprogramme (read-only; am Kessel-Bedienteil einstellbar).
- **Heizkurve** – je Heizkreis Steilheit/Parallelverschiebung/Vorlaufgrenzen plus gemessene
  Außen-↔-Vorlauf-Punktwolke (read-only).
- **Auswertung** – Verlaufsdiagramme und Tagesstatistiken aus einer SQLite-Historie.
- **Parameter** – Nur-Lese-Übersicht aller `par.cgi`-Parameter mit Wert und Grenzen.
- **Loxone** – Klartext-HTTP-Endpunkte und optionaler UDP-Push (siehe [`loxone.md`](loxone.md)).
- **Protokoll** – Audit-Log aller Änderungen (wer/wann/was/Ergebnis), in SQLite.
- **Metriken** – `/metrics` im Prometheus-Format.

Das Web-UI ist ins Binary eingebettet; Verlauf und Audit-Log liegen in einer
SQLite-Datei. Alle Logs gehen nach stdout.

## Schnellstart (Docker)

```bash
mkdir -p data
cp guntacontrol.example.yaml data/guntacontrol.yaml   # Host + writeKey eintragen
docker compose up -d --build
# -> http://localhost:8080
```

Steuerung aktivieren: in `data/guntacontrol.yaml` `allowWrite: true` setzen
(der Schreibschutz lässt sich dann im UI unter ⚙ ausschalten).

## Ohne Docker

```bash
cd web && npx ng build && cd ..            # UI bauen (wird eingebettet)
go build -o guntacontrol ./cmd/guntacontrol
cp guntacontrol.example.yaml guntacontrol.yaml   # anpassen
./guntacontrol                              # liest ./guntacontrol.yaml
```

Details zum Web-UI in [`web/README.md`](web/README.md), zur Loxone-Anbindung in
[`loxone.md`](loxone.md).

## Konfiguration (YAML)

Alle Einstellungen stehen in `guntacontrol.yaml` (Vorlage: `guntacontrol.example.yaml`);
es gibt keine Einzel-Flags mehr. Nur `--config <datei>`, `--data <verzeichnis>` und
`--version` existieren noch als Kommandozeilen-Optionen.

| Schlüssel | Default | Zweck |
|-----------|---------|-------|
| `listen` | `:8080` | Listen-Adresse |
| `dataDir` | `data` | SQLite-Verzeichnis (Verlauf + Audit) |
| `device.host` | `192.168.0.69` | Kessel-IP |
| `device.key` | – | Lese-Key (bei der Flex optional, meist leer) |
| `device.writeKey` | – | Key zum Steuern |
| `device.timeoutSeconds` | `8` | Timeout je Anfrage an den Kessel |
| `pollSeconds` | `10` | Poll-Intervall (min. 5) |
| `retentionDays` | `35` | Aufbewahrung der Verlaufsdaten |
| `allowWrite` | `false` | Steuern überhaupt erlauben |
| `readOnly` | `true` | Schreibschutz (im UI umschaltbar, wird zurückgeschrieben) |
| `overview.circuits` | alle | welche Heizkreise (HK0–HK8) die Startseite zeigt; leer = alle |
| `loxone.udp` | – | UDP-Push an einen Loxone Miniserver (`host:port`) |

Der Schlüssel wird nie im Repo gespeichert: `guntacontrol.yaml` ist gitignored.
Alternativ füllt die Umgebungsvariable `GUNTAMATIC_WRITE_KEY` den Key, falls
`device.writeKey` leer bleibt (siehe `.env.example`).
