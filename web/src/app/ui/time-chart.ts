import { Component, computed, input } from '@angular/core';

export interface ChartSeries {
  label: string;
  color: string;
  unit: string;
  points: { x: number; y: number }[];
}

interface Line {
  label: string;
  color: string;
  path: string;
  last: number | null;
}

/**
 * A small dependency-free line chart over a shared time axis. It draws each series as
 * an SVG polyline inside a fixed 1000×320 viewBox that scales responsively.
 */
@Component({
  selector: 'app-time-chart',
  template: `
    @if (empty()) {
      <div class="empty">Noch keine Verlaufsdaten für diesen Zeitraum.</div>
    } @else {
      <svg viewBox="0 0 1000 320" preserveAspectRatio="none" class="chart">
        @for (g of grid(); track g.y) {
          <line class="grid" x1="48" [attr.y1]="g.y" x2="988" [attr.y2]="g.y" />
          <text class="axis" x="44" [attr.y]="g.y + 4" text-anchor="end">{{ g.label }}</text>
        }
        @for (t of xticks(); track t.x) {
          <text class="axis" [attr.x]="t.x" y="314" text-anchor="middle">{{ t.label }}</text>
        }
        @for (l of lines(); track l.label) {
          <polyline [attr.points]="l.path" [attr.stroke]="l.color" fill="none" stroke-width="2"
            stroke-linejoin="round" stroke-linecap="round" />
        }
      </svg>
      <div class="legend">
        @for (l of lines(); track l.label) {
          <span class="item"><span class="swatch" [style.background]="l.color"></span>{{ l.label }}
            @if (l.last !== null) { <strong>{{ l.last }}°</strong> }
          </span>
        }
      </div>
    }
  `,
  styles: `
    :host { display: block; }
    .chart { width: 100%; height: 300px; display: block; }
    .grid { stroke: var(--line); stroke-width: 1; }
    .axis { fill: var(--muted); font-size: 11px; font-family: inherit; }
    .legend { display: flex; flex-wrap: wrap; gap: 14px; margin-top: 8px; font-size: 13px; }
    .item { display: inline-flex; align-items: center; gap: 6px; color: var(--muted); }
    .item strong { color: var(--ink); }
    .swatch { width: 12px; height: 3px; border-radius: 2px; display: inline-block; }
    .empty { color: var(--muted); padding: 40px 0; text-align: center; }
  `,
})
export class TimeChart {
  readonly series = input<ChartSeries[]>([]);
  readonly from = input<number>(0);
  readonly to = input<number>(0);

  // plot area inside the viewBox
  private readonly left = 48;
  private readonly right = 988;
  private readonly top = 12;
  private readonly bottom = 296;

  protected readonly empty = computed(() => this.series().every((s) => s.points.length === 0));

  private readonly bounds = computed(() => {
    const xs: number[] = [];
    const ys: number[] = [];
    for (const s of this.series()) {
      for (const p of s.points) {
        xs.push(p.x);
        ys.push(p.y);
      }
    }
    const x0 = this.from() || Math.min(...xs);
    const x1 = this.to() || Math.max(...xs);
    let y0 = Math.min(...ys);
    let y1 = Math.max(...ys);
    if (!isFinite(y0) || !isFinite(y1)) {
      y0 = 0;
      y1 = 1;
    }
    if (y0 === y1) {
      y0 -= 1;
      y1 += 1;
    }
    const pad = (y1 - y0) * 0.1;
    return { x0, x1, y0: y0 - pad, y1: y1 + pad };
  });

  private sx(x: number): number {
    const { x0, x1 } = this.bounds();
    const t = x1 === x0 ? 0 : (x - x0) / (x1 - x0);
    return this.left + t * (this.right - this.left);
  }

  private sy(y: number): number {
    const { y0, y1 } = this.bounds();
    const t = y1 === y0 ? 0 : (y - y0) / (y1 - y0);
    return this.bottom - t * (this.bottom - this.top);
  }

  protected readonly lines = computed<Line[]>(() =>
    this.series()
      .filter((s) => s.points.length > 0)
      .map((s) => ({
        label: s.label,
        color: s.color,
        path: s.points.map((p) => `${this.sx(p.x).toFixed(1)},${this.sy(p.y).toFixed(1)}`).join(' '),
        last: s.points.length ? Math.round(s.points[s.points.length - 1].y * 10) / 10 : null,
      })),
  );

  protected readonly grid = computed(() => {
    const { y0, y1 } = this.bounds();
    const out: { y: number; label: string }[] = [];
    for (let i = 0; i <= 4; i++) {
      const val = y0 + ((y1 - y0) * i) / 4;
      out.push({ y: this.sy(val), label: `${Math.round(val)}` });
    }
    return out;
  });

  protected readonly xticks = computed(() => {
    const { x0, x1 } = this.bounds();
    if (!isFinite(x0) || !isFinite(x1)) return [];
    const span = x1 - x0;
    const day = span > 36 * 3600_000;
    const out: { x: number; label: string }[] = [];
    for (let i = 0; i <= 4; i++) {
      const t = x0 + (span * i) / 4;
      const d = new Date(t);
      const label = day
        ? d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit' })
        : d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
      out.push({ x: this.sx(t), label });
    }
    return out;
  });
}
