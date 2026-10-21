import { Component, computed, inject, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { BoilerService } from '../core/api';
import { ToastService } from '../core/toast';
import {
  CIRCUITS,
  HK_PROGRAMS,
  PROGRAM_STATE_CHANNEL,
  PROGRAM_SYN,
  PROGRAMS,
  TILES,
} from '../core/config';
import { ModeOption } from '../core/models';

@Component({
  selector: 'app-overview',
  imports: [DecimalPipe],
  templateUrl: './overview.html',
  styleUrl: './overview.scss',
})
export class OverviewPage {
  protected readonly boiler = inject(BoilerService);
  private readonly toast = inject(ToastService);

  protected readonly PROGRAMS = PROGRAMS;
  protected readonly HK_PROGRAMS = HK_PROGRAMS;
  protected readonly TILES = TILES;
  protected readonly CIRCUITS = CIRCUITS;

  // The synonym currently being written (disables its controls + shows a spinner).
  protected readonly busy = signal<string | null>(null);
  protected readonly showAll = signal(false);

  /** Live main program as text, e.g. "HEIZEN". */
  protected readonly programText = computed(() => this.boiler.text(PROGRAM_STATE_CHANNEL));

  /** Short running state for the hero meta. "Betrieb" is a bool on the Flex, so map it
      to a label instead of showing a raw "true"/"false". */
  protected readonly betriebText = computed(() => {
    const c = this.boiler.channel('Betrieb');
    if (!c) return '';
    if (typeof c.value === 'boolean') return c.value ? 'In Betrieb' : 'Bereit';
    return c.value == null || c.value === '' ? '' : String(c.value);
  });

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
      this.toast.show('err', 'Schreibschutz aktiv – in den Einstellungen deaktivieren');
      return;
    }
    this.busy.set(syn);
    this.toast.clear();
    try {
      const res = await this.boiler.setParam(syn, value);
      if (res.error) {
        this.toast.show('err', res.error);
      } else {
        this.toast.show('ok', `${syn} → ${label} gesetzt`);
      }
    } catch (e) {
      this.toast.show('err', errorText(e));
    } finally {
      this.busy.set(null);
    }
  }

  protected toggleAll(): void {
    this.showAll.update((v) => !v);
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
