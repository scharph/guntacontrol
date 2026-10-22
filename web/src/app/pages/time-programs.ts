import { Component, computed, inject, signal } from '@angular/core';
import { BoilerService } from '../core/api';
import { DaySchedule, Schedule, ScheduleVariant, TimeWindow } from '../core/models';

const KIND_ICON: Record<string, string> = {
  heating: '🔥',
  water: '🚿',
};

@Component({
  selector: 'app-time-programs',
  templateUrl: './time-programs.html',
  styleUrl: './time-programs.scss',
})
export class TimeProgramsPage {
  private readonly boiler = inject(BoilerService);

  protected readonly schedules = signal<Schedule[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal('');

  // Selected weather/season variant per schedule, keyed by title → variant id.
  private readonly selection = signal<Record<string, string>>({});

  constructor() {
    this.reload();
  }

  protected async reload(): Promise<void> {
    this.loading.set(true);
    try {
      const res = await this.boiler.schedules();
      this.schedules.set(res.schedules);
      // Default each schedule to its first variant.
      const sel: Record<string, string> = {};
      for (const s of res.schedules) sel[s.title] = s.variants[0]?.id ?? '';
      this.selection.set(sel);
      this.error.set('');
    } catch {
      this.error.set('Zeitprogramme konnten nicht geladen werden.');
    } finally {
      this.loading.set(false);
    }
  }

  protected icon(kind: string): string {
    return KIND_ICON[kind] ?? '🕑';
  }

  protected selectedId(title: string): string {
    return this.selection()[title] ?? '';
  }

  protected selectVariant(title: string, id: string): void {
    this.selection.update((s) => ({ ...s, [title]: id }));
  }

  protected activeVariant(s: Schedule): ScheduleVariant | undefined {
    const id = this.selectedId(s.title);
    return s.variants.find((v) => v.id === id) ?? s.variants[0];
  }

  /** Minutes-since-midnight → "HH:MM" (1440 → "24:00"). */
  protected fmt(min: number): string {
    const h = Math.floor(min / 60);
    const m = min % 60;
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
  }

  /** Percentage offset/width across a 24 h track. */
  protected pct(min: number): number {
    return (min / 1440) * 100;
  }

  protected label(w: TimeWindow): string {
    return `${this.fmt(w.start)}–${this.fmt(w.end)}`;
  }

  protected dayLabel(d: DaySchedule): string {
    if (!d.windows.length) return 'aus';
    return d.windows.map((w) => this.label(w)).join(', ');
  }
}
