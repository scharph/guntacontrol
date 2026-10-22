// Shape of the Go server's /api/values snapshot (poller.go -> Snapshot/Value).

export type ChannelRaw = number | boolean | string | null;

export interface ChannelValue {
  id: number;
  name: string;
  type: string; // "float" | "bool" | "integer" | "string"
  unit: string;
  value: ChannelRaw;
}

export interface Snapshot {
  updated: string;
  error?: string;
  values: ChannelValue[];
  // Server-level write protection (Schreibschutz) state, mirrored to gate the UI.
  readOnly: boolean;
  // Master capability: whether writes are permitted at all (allowWrite in the config).
  allowWrite: boolean;
  // Heating circuits (0-based) the dashboard should show (overview.circuits in the config).
  circuits?: number[];
  // User-set circuit names from the boiler (HKx11), keyed by circuit index as a string.
  circuitNames?: Record<string, string>;
  // Current program per circuit (HKx01), keyed by circuit index as a string.
  circuitPrograms?: Record<string, number>;
}

export interface ParamAck {
  ack?: string;
  error?: string;
}

// A selectable operating mode for a parameter synonym.
export interface ModeOption {
  value: number;
  label: string;
}

// Definition of a single sensor tile on the dashboard.
export interface TileDef {
  channel: string; // channel name as reported by daqdesc.cgi
  label: string;
  icon: string; // inline emoji/glyph, kept dependency-free
  accent?: string;
}

// ---- Auswertung (evaluation) ----

export interface HistPoint {
  x: number; // unix millis
  y: number;
}

export interface HistSeries {
  channel: string;
  unit: string;
  points: HistPoint[];
}

export interface HistoryResponse {
  from: number;
  to: number;
  series: HistSeries[];
}

export interface DayInsight {
  date: string;
  coverage: number;
  starts?: number;
  runtimeHours?: number;
  avgCycleMinutes?: number;
  outdoor?: number;
  kesselRunning?: number;
  powerRunning?: number;
  bufferTop?: number;
}

export interface Hint {
  level: 'info' | 'warn';
  title: string;
  detail: string;
}

export interface Insights {
  days: DayInsight[];
  hints: Hint[];
}

// ---- Konfigurations-Check (rule-based check of par.cgi against live values) ----

export interface Finding {
  level: 'info' | 'warn';
  title: string;
  detail: string;
  params?: string[]; // par.cgi synonyms the finding refers to
}

export interface ConfigCheckResponse {
  findings: Finding[];
}

// ---- Zeiten (time programs / Uhrenprogramme) ----

export interface TimeWindow {
  start: number; // minutes since midnight
  end: number;
}

export interface DaySchedule {
  day: string; // "Mo".."So"
  windows: TimeWindow[];
}

export interface ScheduleVariant {
  id: string; // par synonym, e.g. "HK003b"
  label: string; // "Normal" | "mild" | "warm" | "Sommer"
  days: DaySchedule[];
}

export interface Schedule {
  title: string; // "Heizkreis 0", "Warmwasser", ...
  kind: 'heating' | 'water';
  variants: ScheduleVariant[];
}

export interface SchedulesResponse {
  schedules: Schedule[];
}

// ---- Heizkurve (heating curve) ----

export interface HeatCurve {
  circuit: number;
  title: string;
  slope: number; // Steilheit
  shift: number; // Parallelverschiebung (K)
  flowMin: number; // Vorlauf min (°C)
  flowMax: number; // Vorlauf max (°C)
  outdoorChannel: string;
  flowChannel: string;
}

export interface HeatCurvesResponse {
  curves: HeatCurve[];
}

// ---- Parameter table (par.cgi) ----

export interface Param {
  syn: string;
  label: string;
  kind: 'number' | 'enum' | 'text';
  value: string;
  unit?: string;
  min?: string;
  max?: string;
  default?: string;
  group: string;
}

export interface ParamsResponse {
  params: Param[];
}

// ---- Audit (change log) ----

export interface AuditEntry {
  time: string;
  who: string;
  via: string; // web | loxone | api | modbus
  action: string; // param | register | settings
  target: string;
  value: string;
  result: string; // ok | error
  detail?: string;
}

export interface AuditResponse {
  entries: AuditEntry[];
}

// Definition of a heating circuit card (status + program control).
export interface CircuitDef {
  index: number; // 0-based circuit index, matched against overview.circuits
  title: string;
  syn: string; // parset synonym, e.g. HK001
  stateChannel: string; // string channel reflecting the live program
  flowChannel: string; // Vorlauf Ist ... temperature channel
}
