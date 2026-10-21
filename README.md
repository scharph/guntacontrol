# guntacontrol

Lokale Überwachung und Steuerung einer **Guntamatic Biostar Flex** über deren
offene HTTP-Schnittstelle – ein kleiner Go-Server mit eingebettetem Angular-Web-UI,
Prometheus-Metriken, SQLite-Verlauf, Loxone-Anbindung und Audit-Log.

## Features

- **Dashboard** (Angular 22, zoneless/signals) – Live-Werte, Betriebsmodus und
  Heizkreis-Programme schalten, mit Schreibschutz.
- **Zeiten** – Anzeige der Uhrenprogramme (read-only; am Kessel-Bedienteil einstellbar).
- **Auswertung** – Verlaufsdiagramme und Tagesstatistiken aus einer SQLite-Historie.
- **Loxone** – Klartext-HTTP-Endpunkte und optionaler UDP-Push (siehe [`loxone.md`](loxone.md)).
- **Protokoll** – Audit-Log aller Änderungen (wer/wann/was/Ergebnis), in SQLite.
- **Metriken** – `/metrics` im Prometheus-Format.

Das Web-UI ist ins Binary eingebettet; Verlauf und Audit-Log liegen in einer
SQLite-Datei. Alle Logs gehen nach stdout.

## Schnellstart (Docker)

```bash
cp .env.example .env     # Kessel-Schlüssel eintragen
docker compose up -d --build
# -> http://localhost:8080
```

Steuerung aktivieren: in `docker-compose.yml` die Zeile `# - -allow-write`
einkommentieren (der Schreibschutz lässt sich dann im UI unter ⚙ ausschalten).

## Ohne Docker

```bash
cd web && npx ng build && cd ..     # UI bauen (wird eingebettet)
go build -o guntacontrol .
./guntacontrol -key $GUNTAMATIC_KEY -write-key $GUNTAMATIC_KEY -allow-write
```

Details zum Web-UI in [`web/README.md`](web/README.md), zur Loxone-Anbindung in
[`loxone.md`](loxone.md).

## Konfiguration (Flags)

| Flag | Default | Zweck |
|------|---------|-------|
| `-host` | `192.168.0.69` | Kessel-IP |
| `-key` | `$GUNTAMATIC_KEY` | Lese-Key (bei der Flex optional) |
| `-write-key` | `$GUNTAMATIC_WRITE_KEY` | Key zum Steuern |
| `-allow-write` | aus | Steuern erlauben (startet mit Schreibschutz AUS) |
| `-addr` | `:8080` | Listen-Adresse |
| `-interval` | `10s` | Poll-Intervall |
| `-db` | `guntacontrol.db` | SQLite (Verlauf + Audit) |
| `-retention` | `35 Tage` | Aufbewahrung der Verlaufsdaten |
| `-loxone-udp` | – | UDP-Push an einen Loxone Miniserver (`host:port`) |

Der Schlüssel wird nie im Repo gespeichert – lokal via `.env` (siehe `.env.example`).
