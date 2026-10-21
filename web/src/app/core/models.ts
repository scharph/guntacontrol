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
  // Master capability: whether writes are permitted at all (server started -allow-write).
  allowWrite: boolean;
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
  kind: 'heating' | 'water' | 'circulation';
  variants: ScheduleVariant[];
}

export interface SchedulesResponse {
  schedules: Schedule[];
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
  title: string;
  syn: string; // parset synonym, e.g. HK001
  stateChannel: string; // string channel reflecting the live program
  flowChannel: string; // Vorlauf Ist ... temperature channel
}
