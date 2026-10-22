import { Component, computed, inject, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { BoilerService } from '../core/api';
import { DayInsight, Finding, HistSeries, Insights } from '../core/models';
import { ChartSeries, TimeChart } from '../ui/time-chart';

interface ChartDef {
  channel: string;
  label: string;
  color: string;
}

const CHART_POINTS: ChartDef[] = [
  { channel: 'Aussentemperatur', label: 'Außen', color: '#3b9ad9' },
  { channel: 'Kesseltemperatur', label: 'Kessel', color: '#e8743b' },
  { channel: 'Puffer oben', label: 'Puffer oben', color: '#e05b5b' },
  { channel: 'Puffer unten', label: 'Puffer unten', color: '#5b7fe0' },
];

const RANGES = [
  { key: '24 h', ms: 24 * 3600_000 },
  { key: '7 Tage', ms: 7 * 24 * 3600_000 },
  { key: '30 Tage', ms: 30 * 24 * 3600_000 },
];

@Component({
  selector: 'app-insights',
  imports: [TimeChart, DecimalPipe],
  templateUrl: './insights.html',
  styleUrl: './insights.scss',
})
export class InsightsPage {
  private readonly boiler = inject(BoilerService);

  protected readonly RANGES = RANGES;
  protected readonly days = signal(14);
  protected readonly rangeMs = signal(RANGES[0].ms);
  protected readonly loading = signal(false);
  protected readonly error = signal('');

  protected readonly data = signal<Insights | null>(null);
  protected readonly findings = signal<Finding[] | null>(null);
  protected readonly checkError = signal('');
  private readonly series = signal<HistSeries[]>([]);
  protected readonly from = signal(0);
  protected readonly to = signal(0);

  protected readonly rows = computed(() => [...(this.data()?.days ?? [])].reverse());
  protected readonly hints = computed(() => this.data()?.hints ?? []);
  protected readonly hasData = computed(() => (this.data()?.days ?? []).some((d) => d.coverage > 0));

  protected readonly chartSeries = computed<ChartSeries[]>(() =>
    CHART_POINTS.map((c) => {
      const s = this.series().find((x) => x.channel === c.channel);
      return {
        label: c.label,
        color: c.color,
        unit: '°C',
        points: s?.points ?? [],
      };
    }),
  );

  constructor() {
    this.reload();
  }

  protected setDays(d: number): void {
    this.days.set(d);
    this.loadInsights();
  }

  protected setRange(ms: number): void {
    this.rangeMs.set(ms);
    this.loadHistory();
  }

  protected reload(): void {
    this.loadInsights();
    this.loadHistory();
    this.loadCheck();
  }

  private async loadCheck(): Promise<void> {
    try {
      this.findings.set((await this.boiler.configCheck()).findings);
      this.checkError.set('');
    } catch {
      this.checkError.set('Konfigurations-Check konnte nicht geladen werden.');
    }
  }

  private async loadInsights(): Promise<void> {
    try {
      this.data.set(await this.boiler.insights(this.days()));
      this.error.set('');
    } catch {
      this.error.set('Auswertung konnte nicht geladen werden.');
    }
  }

  private async loadHistory(): Promise<void> {
    const to = Date.now();
    const from = to - this.rangeMs();
    this.from.set(from);
    this.to.set(to);
    this.loading.set(true);
    try {
      const res = await this.boiler.history(
        CHART_POINTS.map((c) => c.channel),
        from,
        to,
      );
      this.series.set(res.series);
      this.error.set('');
    } catch {
      this.error.set('Verlaufsdaten konnten nicht geladen werden.');
    } finally {
      this.loading.set(false);
    }
  }

  protected day(date: string): string {
    const d = new Date(date + 'T00:00:00');
    return d.toLocaleDateString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit' });
  }

  protected opt(v: number | undefined, suffix = ''): string {
    return v === undefined ? '–' : `${Math.round(v * 10) / 10}${suffix}`;
  }

  protected thin(d: DayInsight): boolean {
    return d.coverage < 0.5;
  }
}
