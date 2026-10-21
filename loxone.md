# guntacontrol – Loxone-Anbindung

Anleitung, um die Guntamatic Biostar Flex über den **guntacontrol**-Server
(`guntamatic.exe`) mit einem **Loxone Miniserver** zu überwachen und zu steuern.

Loxone spricht am einfachsten reines HTTP mit Klartext-Antworten – deshalb bietet der
Server eine eigene `/loxone/...`-Schnittstelle (kein JSON, kein POST nötig) sowie
optional einen UDP-Push.

> Ersetze in allen Beispielen `<SERVER>` durch IP:Port des guntacontrol-Servers
> (z. B. `192.168.0.50:8080`) und `<MINISERVER>` durch die IP des Miniservers.

---

## 1. Server starten

```bash
guntacontrol -key <KEY> -write-key <KEY> -allow-write \
  -loxone-udp <MINISERVER>:7000
```

(oder per Docker Compose, siehe `README`/`docker-compose.yml`.)

| Flag | Zweck |
|------|-------|
| `-allow-write` | **nötig zum Schalten.** Ohne das sind alle Schreibbefehle gesperrt. |
| `-loxone-udp <MINISERVER>:7000` | optionaler UDP-Push (siehe 3). Weglassen = nur HTTP-Polling. |

Das Web-UI ist ins Programm eingebettet (kein `-webroot` nötig); Verlauf und
Audit-Log liegen in der SQLite-DB (`-db`, Standard `guntacontrol.db`).

Zusätzlich muss der **Schreibschutz** aus sein (in der Web-App unter ⚙ → Sicherheit,
oder von Beginn an durch `-allow-write`). Bei aktivem Schreibschutz liefert jeder
Schreibbefehl `ERR: Schreibschutz aktiv`.

---

## 2. Werte lesen – Virtueller HTTP Eingang

Der Endpoint `/loxone/status` liefert alle Kanäle als `Schlüssel=Wert`-Zeilen:

```
online=1
updated=1792589360
readonly=0
Aussentemperatur=18.4
Kesseltemperatur=72.3
Pufferoben=55.1
Leistung=45
...
```

Bool-Werte kommen als `1`/`0`. Die **vollständige, aktuelle Schlüsselliste** zeigt der
Server jederzeit unter `http://<SERVER>/loxone` an.

### Einrichtung in Loxone Config

1. Peripherie → **Virtueller HTTP Eingang** anlegen.
   - **Adresse:** `http://<SERVER>/loxone/status`
   - **Zyklisches Auslesen:** z. B. alle 30 s (oder passend zum Poll-Intervall).
2. Pro gewünschtem Wert einen **Virtuellen HTTP Eingang Befehl** hinzufügen:
   - **Befehlserkennung (Werterkennung):** der Schlüssel **mit** `=`, z. B.
     `Kesseltemperatur=`
   - Als Zahl parsen (Komma/Dezimalpunkt: der Server liefert `.`).

Ein einziges Auslesen füllt so beliebig viele Eingänge – effizient.

> **Alternative – ein Wert pro URL** (ohne Werterkennung):
> `http://<SERVER>/loxone/get?name=Kesseltemperatur` liefert nur die nackte Zahl
> `72.3`. Praktisch, wenn du nur wenige Werte brauchst.

---

## 3. Werte lesen – Virtueller UDP Eingang (Push, empfohlen bei vielen Werten)

Mit `-loxone-udp <MINISERVER>:7000` schickt der Server bei **jedem Poll** dieselben
`Schlüssel=Wert`-Zeilen als UDP-Datagramme an den Miniserver. Kein Polling nötig,
geringere Last.

### Einrichtung in Loxone Config

1. Peripherie → **Virtueller UDP Eingang** anlegen.
   - **Empfangsport:** `7000` (muss zum `-loxone-udp`-Port passen).
2. Pro Wert einen **Befehl** mit Befehlserkennung `Schlüssel=` anlegen – genau wie
   beim HTTP-Eingang (Abschnitt 2).

Die Daten werden auf Pakete < 1200 Byte aufgeteilt, ohne eine `Schlüssel=Wert`-Zeile
zu zerreißen.

---

## 4. Steuern – Virtueller Ausgang

Schreiben geht per einfachem GET auf `/loxone/set`:

```
http://<SERVER>/loxone/set?syn=<SYNONYM>&value=<WERT>
```

Antwort: `OK` / Bestätigungstext oder `ERR: ...`.

### Einrichtung in Loxone Config

1. Peripherie → **Virtueller Ausgang** anlegen, **Adresse:** `http://<SERVER>`
2. **Virtueller Ausgang Befehl** hinzufügen:
   - **Befehl bei EIN:** `/loxone/set?syn=PR001&value=3`  (fester Wert), **oder**
   - mit Platzhalter für analoge Werte: `/loxone/set?syn=PR001&value=<v>`
     (Loxone ersetzt `<v>` durch den Ausgangswert)
   - **HTTP-Methode:** GET

### Schaltbare Parameter (Synonyme)

**`PR001` – Betriebsmodus (Reglerprogramm):**

| Wert | Bedeutung |
|------|-----------|
| 0 | AUS |
| 1 | NORMAL (nach Uhrenprogramm) |
| 2 | WARMWASSER |
| 3 | HEIZEN |
| 4 | ABSENKEN |
| 5 | ABSENKEN BIS |
| 6 | WW Nachladen |
| 7 | ROSTREINIGUNG |
| 8 | HANDBETRIEB |

**`HK001` / `HK101` / `HK201` – Heizkreis 0 / 1 / 2:**

| Wert | Bedeutung |
|------|-----------|
| 0 | AUS |
| 1 | NORMAL (nach Uhrenprogramm) |
| 2 | HEIZEN |
| 3 | ABSENKEN |
| 4 | ABSENKEN BIS |

**Weitere:** `BK006`/`BK106`/`BK206` = Warmwasser-Nachladung (`value=1`),
`ZK006`/… = Zusatz-Warmwasser-Nachladung (`value=1`).

> Die **Heizzeiten (Uhrenprogramme) selbst** sind über diese Schnittstelle
> **nicht** änderbar (Gerätegrenze) – nur die Modus-Umschaltung. Mit
> `HK001=1` (NORMAL) fährt der Heizkreis nach dem am Kessel hinterlegten Uhrenprogramm.

### Beispiele

```
http://<SERVER>/loxone/set?syn=PR001&value=3    # Heizen an
http://<SERVER>/loxone/set?syn=PR001&value=1    # zurück auf Normal (Uhrenprogramm)
http://<SERVER>/loxone/set?syn=HK001&value=2    # Heizkreis 0 dauerhaft HEIZEN
http://<SERVER>/loxone/set?syn=HK001&value=0    # Heizkreis 0 AUS
```

---

## 5. Audit-Log – wer hat wann was geändert

Jede Änderung (egal ob aus Loxone, Web-App oder sonstiger API) wird protokolliert:

```bash
GET http://<SERVER>/api/audit?limit=100
```

```json
{
  "entries": [
    {
      "time": "2026-10-21T15:28:57+02:00",
      "who": "192.168.0.10",
      "via": "loxone",
      "action": "param",
      "target": "PR001",
      "value": "3",
      "result": "ok",
      "detail": "..."
    }
  ]
}
```

- `who` = Client-IP (bei Loxone die Miniserver-IP),
- `via` = `loxone` / `web` / `api`,
- `target`/`value` = Synonym und gesetzter Wert, `result` = `ok`/`error`.

Der Audit-Log wird dauerhaft in der SQLite-DB (`-db`, Tabelle `audit`) gespeichert und
ist im Web-UI unter dem Tab **Protokoll** sichtbar.

---

## 6. Wichtige Kanal-Schlüssel (Auszug)

Vollständige Liste live unter `http://<SERVER>/loxone`.

| Schlüssel | Bedeutung |
|-----------|-----------|
| `Aussentemperatur` | Außentemperatur °C |
| `Kesseltemperatur` | Kesseltemperatur °C |
| `Kesselsolltemp` | Kessel-Solltemperatur °C |
| `Leistung` | Kesselleistung % |
| `Pufferoben` / `Puffermitte` / `Pufferunten` | Pufferspeicher-Temperaturen °C |
| `Pufferladung` | Pufferladezustand % |
| `Warmwasser0` / `Warmwasser1` / `Warmwasser2` | WW-Speicher-Temperaturen °C |
| `VorlaufIst1` / `VorlaufSoll1` | Heizkreis 1 Vorlauf Ist/Soll °C |
| `VorlaufIst2` / `VorlaufSoll2` | Heizkreis 2 Vorlauf Ist/Soll °C |
| `RaumtempHK0` / `RaumtempHK1` / `RaumtempHK2` | Raumtemperaturen Heizkreise °C |
| `Heizkreis0` / `Heizkreis1` / `Heizkreis2` | Heizkreispumpe EIN(1)/AUS(0) |
| `Programm` | aktiver Betriebsmodus (Text) |
| `Kesselfreigabe` | Kesselfreigabe |
| `Fuellstand` | Pellet-Füllstand % |
| `Stoerung0` / `Stoerung1` | Störung aktiv (1/0) |
| `Betriebszeit` / `Servicezeit` | Betriebs-/Servicestunden |
| `online` | 1 = letzter Poll erfolgreich |
| `readonly` | 1 = Schreibschutz aktiv |

---

## 7. Fehlersuche

| Problem | Ursache / Lösung |
|---------|------------------|
| `ERR: writes disabled; start with -allow-write` | Server ohne `-allow-write` gestartet. |
| `ERR: Schreibschutz aktiv` | Schreibschutz in der Web-App (⚙ → Sicherheit) deaktivieren. |
| `ERR: ... Parameter konnte nicht gespeichert werden!` | Ungültiger Wert/Synonym – Wertebereich prüfen (Tabellen oben). |
| Loxone liest keine Werte | Befehlserkennung muss den Schlüssel **inkl. `=`** enthalten (z. B. `Kesseltemperatur=`); Groß-/Kleinschreibung beachten. |
| UDP kommt nicht an | Port in `-loxone-udp` und im Virtuellen UDP Eingang müssen gleich sein; Firewall prüfen. |
| Umlaute im Schlüssel | Schlüssel sind ASCII-gefaltet: `Außentemperatur` → `Aussentemperatur`, „Puffer oben" → `Pufferoben`. Immer die Schreibweise aus `/loxone` verwenden. |
