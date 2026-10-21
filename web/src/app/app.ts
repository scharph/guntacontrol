import { Component, computed, inject, signal } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { BoilerService } from './boiler.service';
import {
  CIRCUITS,
  HK_PROGRAMS,
  POLL_INTERVAL_MS,
  PROGRAM_STATE_CHANNEL,
  PROGRAM_SYN,
  PROGRAMS,
  TILES,
} from './config';
import { ModeOption } from './models';
import { InsightsPage } from './insights';
import { SchedulePage } from './schedule';
import { LoxoneHelp } from './loxone-help';
import { AuditLogPage } from './audit-log';

interface Toast {
  kind: 'ok' | 'err';
  text: string;
}

@Component({
  selector: 'app-root',
  imports: [DecimalPipe, DatePipe, InsightsPage, SchedulePage, LoxoneHelp, AuditLogPage],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App {
  protected readonly boiler = inject(BoilerService);

  protected readonly PROGRAMS = PROGRAMS;
  protected readonly HK_PROGRAMS = HK_PROGRAMS;
  protected readonly TILES = TILES;
  protected readonly CIRCUITS = CIRCUITS;
  protected readonly pollSeconds = POLL_INTERVAL_MS / 1000;

  // The synonym currently being written (disables its controls + shows a spinner).
  protected readonly busy = signal<string | null>(null);
  protected readonly toast = signal<Toast | null>(null);
  protected readonly showAll = signal(false);
  protected readonly showSettings = signal(false);
  protected readonly view = signal<
    'dashboard' | 'insights' | 'schedule' | 'loxone' | 'audit'
  >('dashboard');

  /** Live main program as text, e.g. "HEIZEN". */
  protected readonly programText = computed(() => this.boiler.text(PROGRAM_STATE_CHANNEL));

  /** The PROGRAMS entry whose label matches the live program text (for highlighting). */
  protected readonly activeProgram = computed(() => matchMode(this.programText(), PROGRAMS));

  // The "Heizkreis N" daqdata channel is a bool (circuit pump EIN/AUS), not a text
  // program state, so it reflects activity rather than the selected HK program.
  protected circuitActive(stateChannel: string): boolean {
    return this.boiler.channel(stateChannel)?.value === true;
  }

  protected tileValue(channel: string): string {
    const n = this.boiler.num(channel);
    if (n != null) return n.toFixed(1);
    const t = this.boiler.text(channel);
    return t || '–';
  }

  protected tileUnit(channel: string): string {
    return this.boiler.channel(channel)?.unit ?? '';
  }

  protected async setProgram(value: number): Promise<void> {
    await this.write(PROGRAM_SYN, value, labelOf(PROGRAMS, value));
  }

  protected async setCircuit(syn: string, value: number): Promise<void> {
    await this.write(syn, value, labelOf(HK_PROGRAMS, value));
  }

  private async write(syn: string, value: number, label: string): Promise<void> {
    if (this.busy()) return;
    if (!this.boiler.canWrite()) {
      this.toast.set({ kind: 'err', text: 'Schreibschutz aktiv – in den Einstellungen deaktivieren' });
      setTimeout(() => this.toast.set(null), 4000);
      return;
    }
    this.busy.set(syn);
    this.toast.set(null);
    try {
      const res = await this.boiler.setParam(syn, value);
      if (res.error) {
        this.toast.set({ kind: 'err', text: res.error });
      } else {
        this.toast.set({ kind: 'ok', text: `${syn} → ${label} gesetzt` });
      }
    } catch (e) {
      this.toast.set({ kind: 'err', text: errorText(e) });
    } finally {
      this.busy.set(null);
      setTimeout(() => this.toast.set(null), 4000);
    }
  }

  protected toggleAll(): void {
    this.showAll.update((v) => !v);
  }

  protected toggleSettings(): void {
    this.showSettings.update((v) => !v);
  }

  /**
   * Toggles write protection. Like the Heliotherm app, switching it OFF asks for
   * confirmation first, since it then allows changes to the boiler.
   */
  protected async toggleReadOnly(): Promise<void> {
    const currentlyProtected = this.boiler.readOnly();
    if (currentlyProtected) {
      const ok = confirm(
        'Schreibschutz ausschalten? Danach können Einstellungen am Kessel geändert werden.',
      );
      if (!ok) return;
    }
    try {
      await this.boiler.setReadOnly(!currentlyProtected);
      this.toast.set({
        kind: 'ok',
        text: currentlyProtected ? 'Schreibschutz aus' : 'Schreibschutz ein',
      });
    } catch (e) {
      this.toast.set({ kind: 'err', text: errorText(e) });
    } finally {
      setTimeout(() => this.toast.set(null), 4000);
    }
  }

  protected formatValue(value: unknown, unit: string): string {
    if (typeof value === 'boolean') return value ? 'EIN' : 'AUS';
    if (typeof value === 'number') return `${value}${unit ? ' ' + unit : ''}`;
    return value == null || value === '' ? '–' : String(value);
  }
}

function labelOf(modes: ModeOption[], value: number): string {
  return modes.find((m) => m.value === value)?.label ?? String(value);
}

// Matches a live state string ("HEIZEN", "Normal", ...) to a mode option by label,
// case-insensitively and tolerant of the device's abbreviations.
function matchMode(state: string, modes: ModeOption[]): ModeOption | undefined {
  const s = state.trim().toUpperCase();
  if (!s) return undefined;
  return modes.find((m) => {
    const l = m.label.toUpperCase();
    return l === s || s.startsWith(l) || l.startsWith(s);
  });
}

function errorText(e: unknown): string {
  if (e instanceof HttpErrorResponse) {
    const body = e.error;
    if (body && typeof body === 'object' && 'error' in body) return String((body as { error: unknown }).error);
    if (e.status === 0) return 'Server nicht erreichbar';
    return `HTTP ${e.status}`;
  }
  return 'Unbekannter Fehler';
}
