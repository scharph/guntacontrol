import { Component, computed, inject, signal } from '@angular/core';
import { BoilerService } from '../core/api';
import { HeatCurve, HistSeries } from '../core/models';

// Chart geometry (SVG units). Outdoor axis runs warm (left) to cold (right), like the
// classic heating-curve plot.
const W = 520;
const H = 240;
const PAD = { l: 40, r: 14, t: 12, b: 28 };
const X0 = 20; // left edge: +20 °C outdoor
const X1 = -15; // right edge: -15 °C outdoor (design temp)

interface Dot {
  cx: number;
  cy: number;
}
interface Tick {
  p: number;
  label: string;
}

// Per-circuit view model with all coordinates pre-computed, so the template only draws.
export interface CurveVM {
  curve: HeatCurve;
  dots: Dot[];
  count: number;
  guide: string; // "x1,y1 x2,y2" — linear reference between flowMin and flowMax
  now: { cx: number; cy: number; at: number; flow: number } | null;
  xticks: Tick[];
  yticks: Tick[];
}

@Component({
  selector: 'app-heat-curve',
  templateUrl: './heat-curve.html',
  styleUrl: './heat-curve.scss',
})
export class HeatCurvePage {
  protected readonly boiler = inject(BoilerService);

  protected readonly W = W;
  protected readonly H = H;
  protected readonly PAD = PAD;

  protected readonly curves = signal<HeatCurve[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal('');
  private readonly series = signal<HistSeries[]>([]);
  // History window used to build the measured cloud.
  protected readonly days = signal(14);

  constructor() {
    this.reload();
  }

  protected async reload(): Promise<void> {
    this.loading.set(true);
    try {
      const curves = (await this.boiler.heatCurves()).curves;
      this.curves.set(curves);

      const channels = ['Aussentemperatur', ...new Set(curves.map((c) => c.flowChannel))];
      const to = Date.now();
      const from = to - this.days() * 24 * 3600_000;
      this.series.set((await this.boiler.history(channels, from, to)).series);
      this.error.set('');
    } catch {
      this.error.set('Heizkurve konnte nicht geladen werden.');
    } finally {
      this.loading.set(false);
    }
  }

  // ---- chart geometry ----

  private sx(at: number): number {
    const t = (X0 - at) / (X0 - X1);
    return PAD.l + t * (W - PAD.l - PAD.r);
  }
  private sy(v: number, y0: number, y1: number): number {
    const c = Math.max(y0, Math.min(y1, v));
    const t = y1 === y0 ? 0 : (c - y0) / (y1 - y0);
    return H - PAD.b - t * (H - PAD.t - PAD.b);
  }

  protected readonly vms = computed<CurveVM[]>(() => {
    const outdoor = this.series().find((s) => s.channel === 'Aussentemperatur');
    const byTs = new Map<number, number>();
    for (const p of outdoor?.points ?? []) byTs.set(p.x, p.y);

    return this.curves().map((curve) => {
      const y0 = Math.floor((curve.flowMin - 5) / 5) * 5;
      const y1 = Math.ceil((curve.flowMax + 5) / 5) * 5;

      const flow = this.series().find((s) => s.channel === curve.flowChannel);
      const dots: Dot[] = [];
      for (const p of flow?.points ?? []) {
        const at = byTs.get(p.x);
        if (at === undefined) continue;
        if (p.y <= 0) continue; // pump off / no flow: not part of the curve
        dots.push({ cx: this.sx(at), cy: this.sy(p.y, y0, y1) });
      }

      // Linear reference guide (NOT the controller's internal curve): flowMin at the
      // warm end, flowMax at the design temperature.
      const guide =
        `${this.sx(X0)},${this.sy(curve.flowMin, y0, y1)} ` +
        `${this.sx(X1)},${this.sy(curve.flowMax, y0, y1)}`;

      const atNow = this.boiler.num('Aussentemperatur');
      const flowNow = this.boiler.num(curve.flowChannel);
      const now =
        atNow != null && flowNow != null && flowNow > 0
          ? { cx: this.sx(atNow), cy: this.sy(flowNow, y0, y1), at: atNow, flow: flowNow }
          : null;

      const xticks: Tick[] = [20, 15, 10, 5, 0, -5, -10, -15].map((x) => ({
        p: this.sx(x),
        label: String(x),
      }));
      const yticks: Tick[] = [];
      for (let v = y0; v <= y1; v += Math.max(5, Math.round((y1 - y0) / 5 / 5) * 5)) {
        yticks.push({ p: this.sy(v, y0, y1), label: String(v) });
      }

      return { curve, dots, count: dots.length, guide, now, xticks, yticks };
    });
  });

  protected fmt(v: number, d = 1): string {
    return v.toFixed(d);
  }
}

