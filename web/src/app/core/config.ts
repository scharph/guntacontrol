import { CircuitDef, ModeOption, TileDef } from './models';

// PR001 "Programm" — the boiler's main operating mode. Values/labels come straight
// from the device's own par.cgi enum table.
export const PROGRAMS: ModeOption[] = [
  { value: 0, label: 'Aus' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'Warmwasser' },
  { value: 3, label: 'Heizen' },
  { value: 4, label: 'Absenken' },
  { value: 5, label: 'Absenken bis' },
  { value: 6, label: 'WW Nachlauf' },
  { value: 7, label: 'Rostreinigung' },
  { value: 8, label: 'Handbetrieb' },
];

// HK0xx "Progamm HKx" — per heating circuit (par.cgi: min 1, max 4; 0 = Aus).
export const HK_PROGRAMS: ModeOption[] = [
  { value: 0, label: 'Aus' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'Heizen' },
  { value: 3, label: 'Absenken' },
  { value: 4, label: 'Absenken bis' },
];

export const TILES: TileDef[] = [
  { channel: 'Betrieb', label: 'Betrieb', icon: '⚡', accent: '#2fa98b' },
  { channel: 'Aussentemperatur', label: 'Außen', icon: '🌡️', accent: '#3b9ad9' },
  { channel: 'Kesseltemperatur', label: 'Kessel', icon: '🔥', accent: '#e8743b' },
  { channel: 'Puffer oben', label: 'Puffer oben', icon: '🟥', accent: '#e05b5b' },
  { channel: 'Puffer unten', label: 'Puffer unten', icon: '🟦', accent: '#5b7fe0' },
  { channel: 'Pufferladung', label: 'Pufferladung', icon: '🔋', accent: '#2fa98b' },
  { channel: 'Warmwasser 0', label: 'Warmwasser', icon: '🚿', accent: '#2f9fa9' },
  { channel: 'Leistung', label: 'Leistung', icon: '⚙️', accent: '#8a7fd0' },
  { channel: 'CO2 Gehalt', label: 'CO₂', icon: '💨', accent: '#7a8a99' },
  { channel: 'Asche leeren in', label: 'Asche leeren in', icon: '🧹', accent: '#b08968' },
];

export const CIRCUITS: CircuitDef[] = [
  { index: 0, title: 'Heizkreis 0', syn: 'HK001', stateChannel: 'Heizkreis 0', flowChannel: 'Vorlauf Ist 0' },
  { index: 1, title: 'Heizkreis 1', syn: 'HK101', stateChannel: 'Heizkreis 1', flowChannel: 'Vorlauf Ist 1' },
  { index: 2, title: 'Heizkreis 2', syn: 'HK201', stateChannel: 'Heizkreis 2', flowChannel: 'Vorlauf Ist 2' },
  { index: 3, title: 'Heizkreis 3', syn: 'HK301', stateChannel: 'Heizkreis 3', flowChannel: 'Vorlauf Ist 3' },
  { index: 4, title: 'Heizkreis 4', syn: 'HK401', stateChannel: 'Heizkreis 4', flowChannel: 'Vorlauf Ist 4' },
];

// Channel that carries the live main program as text (daqdesc id 70).
export const PROGRAM_STATE_CHANNEL = 'Programm';
export const PROGRAM_SYN = 'PR001';

export const POLL_INTERVAL_MS = 5000;
