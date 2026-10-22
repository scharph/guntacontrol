import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

interface Tip {
  title: string;
  text: string;
  // par.cgi synonyms the tip refers to, shown as hints for the Parameter page.
  params?: string[];
}

interface TipSection {
  icon: string;
  title: string;
  tips: Tip[];
}

// Best practices for pellet boilers collected from the community (HaustechnikDialog,
// energiesparhaus.at) and manufacturer guidance. Static content; the live checks of
// this boiler's settings are on the Auswertung page.
const SECTIONS: TipSection[] = [
  {
    icon: '🔥',
    title: 'Brenner & Puffer – wenige, lange Läufe',
    tips: [
      {
        title: 'Starts sind der größte Hebel',
        text: 'Jede Zündung braucht 250–400 W über mehrere Minuten, verursacht den Großteil der Emissionen und nutzt das Zündelement ab. Richtwert: im Winter ≤ 5 Starts pro Tag bei ≥ 2–3 h pro Lauf. Die Ampel auf der Auswertung zeigt, wie die Anlage dasteht.',
      },
      {
        title: 'Puffer voll durchladen statt nachheizen',
        text: 'Eine große Spreizung zwischen Lade-Ein und Lade-Aus führt zu langen Brennerläufen. Eine knappe Teilladung lässt den Kessel öfter starten.',
        params: ['P0003', 'P0004', 'P0004b'],
      },
      {
        title: 'Puffertemperatur so niedrig wie möglich',
        text: 'Der Puffer muss nur so heiß sein, wie der höchste tatsächliche Bedarf verlangt – bei Fußbodenheizung und Warmwasser meist 55–60 °C oben. Ungenutzte, aber aktive Heizkreise mit hohem Vorlauf-Minimum halten ihn unnötig hoch.',
        params: ['HPx02', 'HPx05'],
      },
      {
        title: 'Schichtung im Puffer erhalten',
        text: 'Zu viel Durchfluss durch die Pufferpumpe vermischt den Speicher – der Kessel startet dann öfter und mit höherer Rücklauftemperatur. Oben und unten sollten deutlich (≥ 10 K) auseinanderliegen.',
      },
    ],
  },
  {
    icon: '🌡️',
    title: 'Heizkreise',
    tips: [
      {
        title: 'Hydraulischer Abgleich',
        text: 'Bei Fußbodenheizung die Raum-Stellventile weit offen lassen und die Raumtemperatur über die Heizkurve regeln, nicht über die Thermostate in den Räumen.',
      },
      {
        title: 'Heizkurve schrittweise absenken',
        text: 'Steilheit oder Parallelverschiebung in kleinen Schritten senken und jeweils einige Tage beobachten – so niedrig, dass es gerade noch warm genug ist. Läuft ein Kreis bei Frost am Vorlauf-Maximum, das Maximum anheben (Estrich-Grenze beachten), nicht die Steilheit.',
        params: ['HKx07', 'HPx09', 'HPx06'],
      },
      {
        title: 'Fußbodenheizung: wenig oder keine Nachtabsenkung',
        text: 'Estrich ist träge – das Wiederaufheizen frisst den Großteil der Ersparnis. 2–3 K Absenkung reichen, oft ist durchlaufen sogar günstiger.',
        params: ['HKx04', 'HKx05'],
      },
      {
        title: 'Heizgrenze 15–16 °C',
        text: 'Für Flächenheizung in einem gedämmten Haus ist eine Abschaltung bei 18 °C Außentemperatur hoch. 15–16 °C sparen in der Übergangszeit spürbar Pellets.',
        params: ['HKx09'],
      },
      {
        title: 'Selten genutzte Kreise auf AUS',
        text: 'Ein Heizkörperkreis, der nur gelegentlich gebraucht wird, sollte auf AUS stehen statt in Dauerabsenkung – und bei Bedarf auf HEIZEN geschaltet werden. Den Frostschutz übernimmt die Regelung trotzdem.',
        params: ['HKx01'],
      },
    ],
  },
  {
    icon: '🚿',
    title: 'Sommer & Warmwasser',
    tips: [
      {
        title: 'Ein Ladefenster pro Tag',
        text: 'Im Sommer Warmwasser nur einmal täglich laden, am besten am späten Nachmittag. Jedes zusätzliche Fenster bedeutet einen weiteren kurzen Brennerlauf.',
        params: ['BK002', 'BK003'],
      },
      {
        title: 'Kessel im Sommer möglichst aus',
        text: 'Pellets nur für Warmwasser sind ineffizient – Bereitschaftsverluste und Starts dominieren. Bewährt: Heizstab mit PV-Überschuss, Solarthermie oder eine Warmwasser-Wärmepumpe. Damit kann der Kessel von Mai bis September oft ganz aus bleiben.',
      },
      {
        title: 'Warmwasser 55 °C plus Legionellen-Zyklus',
        text: 'Statt dauerhaft ≥ 60 °C den Speicher auf 55 °C halten und einmal pro Woche auf 60–65 °C aufheizen. Bei großen Anlagen bzw. Mietobjekten die lokalen Hygienevorgaben beachten.',
        params: ['BK004'],
      },
    ],
  },
  {
    icon: '🪵',
    title: 'Pellets & Lager',
    tips: [
      {
        title: 'ENplus-A1 mit wenig Feinanteil',
        text: 'Auf zertifizierte Ware achten und beim Lieferanten nach Absiebung fragen. Feinanteil verursacht Förderstörungen, Schlacke und höhere Emissionen.',
      },
      {
        title: 'Kessel vor dem Einblasen ausschalten',
        text: 'Etwa 1 h vor der Befüllung den Kessel abschalten (Rückbrandgefahr, Unterdruck im Lager). Die Regelung bietet dafür eine Sperrzeit-Befüllung.',
        params: ['K0007b'],
      },
      {
        title: 'Lager alle 2–3 Jahre leerfahren',
        text: 'Den Lagerraum vollständig leeren und den angesammelten Feinanteil entfernen, bevor neu befüllt wird.',
      },
      {
        title: 'CO-Gefahr im Lagerraum',
        text: 'Pellets gasen Kohlenmonoxid aus. Vor dem Betreten gründlich lüften, idealerweise mit CO-Melder – und nie allein in ein geschlossenes Lager steigen.',
      },
      {
        title: 'Im Frühsommer kaufen',
        text: 'Pelletpreise sind meist von Mai bis Juli am niedrigsten. Den Verbrauch (kg pro Heizgradtag auf der Auswertung) im Blick behalten, um die Menge zu planen.',
      },
    ],
  },
  {
    icon: '🧰',
    title: 'Wartung',
    tips: [
      {
        title: 'Asche & Wärmetauscher',
        text: 'Asche rechtzeitig leeren und die automatische Wärmetauscher-Reinigung kontrollieren – Belag auf den Heizflächen erhöht die Abgastemperatur und senkt den Wirkungsgrad.',
        params: ['K0003'],
      },
      {
        title: 'Jährlicher Service',
        text: 'Lambdasonde, Zündelement, Dichtungen, Saugzug und Brennraum prüfen lassen und den Service-Zähler danach zurücksetzen.',
      },
      {
        title: 'Kamin',
        text: 'Zugbegrenzer prüfen und auf Kondensat bzw. Versottung achten – besonders bei vielen kurzen Teillast-Läufen.',
      },
    ],
  },
];

/** Static best-practice tips for operating a pellet boiler. */
@Component({
  selector: 'app-tips',
  imports: [RouterLink],
  templateUrl: './tips.html',
  styleUrl: './tips.scss',
})
export class TipsPage {
  protected readonly sections = SECTIONS;
}
