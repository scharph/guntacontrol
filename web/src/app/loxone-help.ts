import { Component, computed, inject } from '@angular/core';
import { BoilerService } from './boiler.service';

/** Mirrors the server's loxoneKey(): fold umlauts, drop non-alphanumerics. */
function loxKey(name: string): string {
  const folded = name
    .replace(/ä/g, 'ae')
    .replace(/ö/g, 'oe')
    .replace(/ü/g, 'ue')
    .replace(/Ä/g, 'Ae')
    .replace(/Ö/g, 'Oe')
    .replace(/Ü/g, 'Ue')
    .replace(/ß/g, 'ss');
  return Array.from(folded)
    .filter((c) => /[A-Za-z0-9]/.test(c))
    .join('');
}

interface Mode {
  value: number;
  label: string;
}

@Component({
  selector: 'app-loxone-help',
  templateUrl: './loxone-help.html',
  styleUrl: './loxone-help.scss',
})
export class LoxoneHelp {
  protected readonly boiler = inject(BoilerService);

  /** Server base the browser reached guntacontrol on, used in copy-ready URLs. */
  protected readonly host = location.host;

  protected readonly programModes: Mode[] = [
    { value: 0, label: 'AUS' },
    { value: 1, label: 'NORMAL (Uhrenprogramm)' },
    { value: 2, label: 'WARMWASSER' },
    { value: 3, label: 'HEIZEN' },
    { value: 4, label: 'ABSENKEN' },
    { value: 5, label: 'ABSENKEN BIS' },
    { value: 6, label: 'WW Nachladen' },
    { value: 7, label: 'ROSTREINIGUNG' },
    { value: 8, label: 'HANDBETRIEB' },
  ];

  protected readonly circuitModes: Mode[] = [
    { value: 0, label: 'AUS' },
    { value: 1, label: 'NORMAL (Uhrenprogramm)' },
    { value: 2, label: 'HEIZEN' },
    { value: 3, label: 'ABSENKEN' },
    { value: 4, label: 'ABSENKEN BIS' },
  ];

  /** Live channel keys (as Loxone would see them) from the current snapshot. */
  protected readonly channels = computed(() =>
    this.boiler
      .values()
      .map((v) => ({ key: loxKey(v.name), name: v.name, unit: v.unit }))
      .filter((c) => c.key),
  );

  protected copy(text: string): void {
    navigator.clipboard?.writeText(text).catch(() => {});
  }
}
