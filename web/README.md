# guntacontrol – Steuerungs-App (Angular 22)

Dashboard im Stil der Heliotherm-Web-Control für die **Guntamatic Biostar Flex**
(192.168.0.69). Zeigt Live-Werte (Temperaturen, Puffer, Warmwasser, Leistung …)
und schaltet den **Betriebsmodus** (`PR001`) sowie die **Heizkreis-Programme**
(`HK001/HK101/HK201`).

Technik: Angular 22, **zoneless**, Signals. Lesen über `httpResource`
(`GET /api/values`, Poll alle 5 s), Steuern über `HttpClient` (`POST /api/param`).
Backend ist der Go-Server im übergeordneten Verzeichnis (spricht den Kessel über
`/daqdata.cgi` bzw. `/ext/parset.cgi` an).

## Docker (empfohlen)

Alles in einem Image – UI ist ins Binary eingebettet, Daten (SQLite + Schreibschutz-
Status) im `./data`-Volume, Logs nur nach stdout (keine lokalen Logdateien):

```bash
cp .env.example .env     # Schlüssel eintragen
docker compose up -d --build
# -> http://localhost:8080
```

Steuern aktivieren: in `docker-compose.yml` die Zeile `# - -allow-write` einkommentieren
(der Schreibschutz lässt sich dann im UI unter ⚙ ausschalten).

## Produktiv ohne Docker (ein Binary, kein CORS)

```bash
# 1. UI bauen (wird ins Binary eingebettet)
cd web && npx ng build && cd ..

# 2. Server bauen & starten – UI + API aus einem Prozess, kein -webroot nötig
go build -o guntacontrol .
KEY=<DEIN-SCHLUESSEL>   # 36-stelliger Level-1-Key des Kessels
./guntacontrol -key $KEY -write-key $KEY -allow-write
# -> http://localhost:8080
```

Ohne `-allow-write` ist nur Monitoring aktiv; Steuerbefehle liefern dann 403.

## Loxone-Anbindung

Der Go-Server bietet eine Klartext-HTTP-Schnittstelle für den Loxone Miniserver
(kein JSON/POST nötig):

**Lesen** – Virtueller HTTP Eingang, Abfrageadresse auf `…/loxone/status`, dann je
Wert einen „Virtuellen HTTP Eingang Befehl" mit Befehlserkennung = Schlüssel + `=`
(z. B. `Kesseltemperatur=`):

```
GET http://<host>:8080/loxone/status      # alle Kanäle als Key=Value (+ online/updated/readonly)
GET http://<host>:8080/loxone/get?name=Kesseltemperatur   # nur der nackte Wert, z. B. 72.4
GET http://<host>:8080/loxone             # Hilfe inkl. aller aktuellen Schlüssel
```

**Schreiben** – Virtueller Ausgang, Befehl bei EIN z. B.
`/loxone/set?syn=PR001&value=<v>` (Loxone ersetzt `<v>` durch den Wert):

```
GET http://<host>:8080/loxone/set?syn=PR001&value=3   # Programm HEIZEN
GET http://<host>:8080/loxone/set?syn=HK001&value=1   # Heizkreis 0 = NORMAL (Uhrenprogramm)
```

Antwort ist `OK`/Bestätigungstext oder `ERR: …`. Schreiben erfordert `-allow-write`
und ausgeschalteten Schreibschutz (sonst `ERR`). Schlüssel sind ASCII-gefaltet
(Umlaute → ae/oe/ue, Leerzeichen entfernt: „Puffer oben" → `PufferOben`); die genauen
Namen zeigt `/loxone` bzw. `/loxone/status` live an.

**UDP-Push** (statt Polling) – der Server schickt bei jedem Poll alle Werte als
`Key=Value`-Datagramme an den Miniserver (Loxone: „Virtueller UDP Eingang"):

```bash
./guntacontrol … -loxone-udp 192.168.0.10:7000
```

Zeilen werden auf Datagramme < 1200 Byte aufgeteilt, ohne einen Marker zu zerreißen.

## Speicher (SQLite)

Auswertung (Verlauf) **und** Audit-Log liegen in einer gemeinsamen SQLite-Datenbank
(reiner Go-Treiber `modernc.org/sqlite`, kein C-Compiler nötig):

```bash
./guntacontrol … -db guntacontrol.db        # Standard; Tabellen: samples, audit
```

Die Datei lässt sich mit jedem SQLite-Tool öffnen. `-retention` steuert, wie lange
History-Samples behalten werden (Default 35 Tage).

## Audit-Log (wer hat wann was geändert)

Jede Parameter-/Statusänderung wird protokolliert: Zeit, Client-IP (`who`),
Schnittstelle (`via` = web/loxone/api), Ziel (Synonym/Register/`readOnly`),
angeforderter Wert und Ergebnis (`ok`/`error`).

```
GET http://<host>:8080/api/audit?limit=100     # jüngste Einträge (neueste zuerst)
```

Im Web-UI zeigt der Tab **Protokoll** dieselben Daten. Die Web-App weist sich per
Header `X-Actor: web` als Quelle aus; ohne Header ist `via=api`, Loxone-Schreibvorgänge
erscheinen als `via=loxone`.

## Entwicklung (Hot-Reload)

```bash
# Terminal 1: Go-Server auf :8080 (UI eingebettet, kein -webroot)
./guntacontrol -key $KEY -write-key $KEY -allow-write

# Terminal 2: Angular Dev-Server, /api wird via proxy.conf.json auf :8080 geleitet
cd web && npm start   # -> http://localhost:4200
```

## Struktur

- `src/app/boiler.service.ts` – `httpResource`-Lesekanal + `setParam()`-Schreiben
- `src/app/app.ts` / `app.html` / `app.scss` – Dashboard-Komponente
- `src/app/config.ts` – Modus-Enums (`PR001`, `HK0xx`), Kachel-/Heizkreis-Definitionen
- `src/app/models.ts` – Typen passend zum `/api/values`-Snapshot
