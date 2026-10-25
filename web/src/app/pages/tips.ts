import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BoilerService } from '../core/api';
import { errorText } from '../core/errors';
import { Param } from '../core/models';
import { ToastService } from '../core/toast';

/** One concrete change a tip proposes for this boiler. */
interface TipAction {
  syn: string;
  value: string; // value sent to the boiler (enum index for enums)
  label: string; // e.g. "FBH EG: Heizgrenze 18 → 16 °C"
}

/** Read access to the boiler's current settings, for deriving actions. */
interface TipContext {
  num(syn: string): number | undefined;
  text(syn: string): string;
  circuits: number[]; // active (or configured) heating circuits
  name(c: number): string;
  floorHeating(c: number): boolean;
}

interface Tip {
  title: string;
  text: string;
  // par.cgi synonyms the tip refers to, shown as hints for the Parameter page.
  params?: string[];
  // Changes that would apply the tip here; only those differing from the current value.
  actions?: (ctx: TipContext) => TipAction[];
}

// Flow maximum up to which a circuit is treated as floor heating.
const FLOOR_MAX_FLOW = 45;

// The Level-1 key may only write operating-mode parameters via /ext/parset.cgi: PR001,
// HKx01, K0010 (Kesselfreigabe) and BKx06 (WW Nachladen) are accepted; setpoints like
// BK004/HKx04/HKx05/HKx07 are rejected ("Parameter konnte nicht gespeichert werden!").
// Everything else has to be set on the boiler display.
const WRITABLE = /^(PR001|HK\d01|K0010|BK\d06)$/;

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
        actions: (ctx) =>
          ctx.circuits.filter(ctx.floorHeating).flatMap((c) => {
            const day = ctx.num(`HK${c}04`);
            const night = ctx.num(`HK${c}05`);
            if (day === undefined || night === undefined || day - night <= 3) return [];
            const target = day - 2;
            return [{ syn: `HK${c}05`, value: target.toFixed(1), label: `${ctx.name(c)}: Nacht ${night} → ${target} °C` }];
          }),
      },
      {
        title: 'Heizgrenze 15–16 °C',
        text: 'Für Flächenheizung in einem gedämmten Haus ist eine Abschaltung bei 18 °C Außentemperatur hoch. 15–16 °C sparen in der Übergangszeit spürbar Pellets.',
        params: ['HKx09'],
        actions: (ctx) =>
          ctx.circuits.filter(ctx.floorHeating).flatMap((c) => {
            const limit = ctx.num(`HK${c}09`);
            if (limit === undefined || limit <= 16) return [];
            return [{ syn: `HK${c}09`, value: '16', label: `${ctx.name(c)}: Heizgrenze ${limit} → 16 °C` }];
          }),
      },
      {
        title: 'Selten genutzte Kreise auf AUS',
        text: 'Ein Heizkörperkreis, der nur gelegentlich gebraucht wird, sollte auf AUS stehen statt in Dauerabsenkung – und bei Bedarf auf HEIZEN geschaltet werden. Den Frostschutz übernimmt die Regelung trotzdem.',
        params: ['HKx01'],
        actions: (ctx) =>
          ctx.circuits
            .filter((c) => !ctx.floorHeating(c) && ctx.text(`HK${c}01`) !== 'AUS')
            .map((c) => ({
              syn: `HK${c}01`,
              value: '0', // enum index of AUS
              label: `${ctx.name(c)}: ${ctx.text(`HK${c}01`)} → AUS`,
            })),
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
        actions: (ctx) => {
          const soll = ctx.num('BK004');
          return soll !== undefined && soll > 55
            ? [{ syn: 'BK004', value: '55', label: `Warmwasser-Soll ${soll} → 55 °C` }]
            : [];
        },
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
  protected readonly boiler = inject(BoilerService);
  private readonly toast = inject(ToastService);

  protected readonly sections = SECTIONS;
  private readonly params = signal<Map<string, Param> | null>(null);
  protected readonly busy = signal('');

  private readonly ctx = computed<TipContext | null>(() => {
    const p = this.params();
    if (!p) return null;
    const num = (syn: string) => {
      const v = parseFloat(p.get(syn)?.value ?? '');
      return Number.isNaN(v) ? undefined : v;
    };
    const names = this.boiler.circuitNames() as Record<string, string>;
    return {
      num,
      text: (syn) => p.get(syn)?.value ?? '',
      circuits: this.boiler.circuits(),
      name: (c) => names[String(c)] || `Heizkreis ${c}`,
      floorHeating: (c) => (num(`HP${c}06`) ?? 99) <= FLOOR_MAX_FLOW,
    };
  });

  /** Proposed actions per tip title, recomputed when params or circuits change. */
  protected readonly actions = computed(() => {
    const ctx = this.ctx();
    const out: Record<string, TipAction[]> = {};
    if (!ctx) return out;
    for (const s of SECTIONS) for (const t of s.tips) if (t.actions) out[t.title] = t.actions(ctx);
    return out;
  });

  constructor() {
    this.load();
  }

  private async load(): Promise<void> {
    try {
      const res = await this.boiler.params();
      this.params.set(new Map(res.params.map((p) => [p.syn, p])));
    } catch {
      // The tips stay readable without the actions.
    }
  }

  protected writable(a: TipAction): boolean {
    return WRITABLE.test(a.syn);
  }

  protected hasWritable(title: string): boolean {
    return (this.actions()[title] ?? []).some((a) => this.writable(a));
  }

  /** Sends one tip action to the boiler after confirmation, then re-reads the params. */
  protected async apply(a: TipAction): Promise<void> {
    if (this.busy()) return;
    if (!this.boiler.canWrite()) {
      this.toast.show('err', 'Schreibschutz aktiv – in den Einstellungen deaktivieren');
      return;
    }
    if (!window.confirm(`${a.label}\n\n${a.syn} = ${a.value} an den Kessel senden?`)) return;

    this.busy.set(a.syn);
    this.toast.clear();
    try {
      const res = await this.boiler.setParam(a.syn, a.value);
      if (res.error) {
        this.toast.show('err', `${a.syn}: ${res.error}`);
        return;
      }
      this.toast.show('ok', `${a.label}: gespeichert`);
      await this.load();
    } catch (e) {
      this.toast.show('err', errorText(e));
    } finally {
      this.busy.set('');
    }
  }
}
