import { Component, computed, inject, signal } from '@angular/core';
import { BoilerService } from '../core/api';
import { Param } from '../core/models';

// Friendly German labels for the synonym-prefix groups (UI text stays German).
const GROUP_LABELS: Record<string, string> = {
  PR: 'Programm',
  HK: 'Heizkreise',
  HP: 'Vorlauf / Heizkurve',
  BK: 'Warmwasser',
  ZK: 'Zusatz-Warmwasser',
  PU: 'Puffer',
  SO: 'Solar',
  KT: 'Kessel',
  K: 'Kessel',
  AU: 'Austragung',
  ANL: 'Anlage',
  IO: 'Ein-/Ausgänge',
};

/** Read-only overview of all par.cgi parameters with their current value and limits. */
@Component({
  selector: 'app-parameters',
  templateUrl: './parameters.html',
  styleUrl: './parameters.scss',
})
export class ParametersPage {
  private readonly boiler = inject(BoilerService);

  protected readonly params = signal<Param[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal('');
  protected readonly search = signal('');
  protected readonly group = signal('all');

  constructor() {
    this.reload();
  }

  protected async reload(): Promise<void> {
    this.loading.set(true);
    try {
      this.params.set((await this.boiler.params()).params);
      this.error.set('');
    } catch {
      this.error.set('Parameter konnten nicht geladen werden.');
    } finally {
      this.loading.set(false);
    }
  }

  // Distinct groups present, as {id, label}, for the filter dropdown.
  protected readonly groups = computed(() => {
    const ids = [...new Set(this.params().map((p) => p.group))].sort();
    return ids.map((id) => ({ id, label: this.groupLabel(id) }));
  });

  protected readonly rows = computed(() => {
    const q = this.search().trim().toLowerCase();
    const g = this.group();
    return this.params().filter((p) => {
      if (g !== 'all' && p.group !== g) return false;
      if (!q) return true;
      return p.syn.toLowerCase().includes(q) || p.label.toLowerCase().includes(q);
    });
  });

  protected groupLabel(id: string): string {
    return GROUP_LABELS[id] ?? id;
  }

  protected setSearch(v: string): void {
    this.search.set(v);
  }

  protected limits(p: Param): string {
    if (p.kind === 'number' && p.min !== undefined && p.max !== undefined) {
      return `${p.min} … ${p.max}`;
    }
    if (p.kind === 'enum') return 'Auswahl';
    if (p.kind === 'text') return 'Text';
    return '–';
  }
}
