import { computed, inject, Injectable, Signal } from '@angular/core';
import { HttpClient, httpResource } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import {
  AuditResponse,
  ChannelValue,
  HeatCurvesResponse,
  HistoryResponse,
  Insights,
  ParamAck,
  ParamsResponse,
  SchedulesResponse,
  Snapshot,
} from './models';
import { HttpParams } from '@angular/common/http';
import { POLL_INTERVAL_MS } from './config';

const EMPTY: Snapshot = { updated: '', values: [], readOnly: true, allowWrite: false };

/**
 * BoilerService is the single data layer between the dashboard and the Go backend.
 *
 * Reads use Angular 22's `httpResource`: a reactive GET /api/values whose status and
 * value are exposed as signals. A timer calls `.reload()` so the snapshot refreshes on
 * an interval. Writes go through HttpClient POST /api/param and then force a reload so
 * the UI reflects the new state quickly.
 */
@Injectable({ providedIn: 'root' })
export class BoilerService {
  private readonly http = inject(HttpClient);

  // Reactive GET /api/values. defaultValue keeps value() non-undefined.
  private readonly snapshot = httpResource<Snapshot>(() => '/api/values', {
    defaultValue: EMPTY,
  });

  constructor() {
    setInterval(() => this.snapshot.reload(), POLL_INTERVAL_MS);
  }

  readonly values: Signal<ChannelValue[]> = computed(() => this.snapshot.value().values);
  readonly updated = computed(() => this.snapshot.value().updated);
  readonly loading = computed(() => this.snapshot.isLoading());

  // A transport/HTTP failure or a backend-reported poll error both surface here.
  readonly error = computed(() => {
    if (this.snapshot.error()) return 'Keine Verbindung zum Server';
    return this.snapshot.value().error ?? '';
  });

  readonly online = computed(() => !this.snapshot.error() && !!this.updated());

  // Write protection (Schreibschutz), mirrored from the server.
  readonly readOnly = computed(() => this.snapshot.value().readOnly);
  readonly allowWrite = computed(() => this.snapshot.value().allowWrite);
  // Controls may be used only when online, writes are permitted and protection is off.
  readonly canWrite = computed(() => this.online() && this.allowWrite() && !this.readOnly());

  private readonly byName = computed(() => {
    const m = new Map<string, ChannelValue>();
    for (const v of this.values()) m.set(v.name, v);
    return m;
  });

  channel(name: string): ChannelValue | undefined {
    return this.byName().get(name);
  }

  /** Numeric value for a channel, or null if it is not a number. */
  num(name: string): number | null {
    const v = this.byName().get(name)?.value;
    return typeof v === 'number' ? v : null;
  }

  /** Text/state value for a channel (e.g. "HEIZEN", "EIN"). */
  text(name: string): string {
    const v = this.byName().get(name)?.value;
    return v == null ? '' : String(v);
  }

  /** Writes a control parameter by synonym, then reloads the snapshot. */
  async setParam(syn: string, value: number | string): Promise<ParamAck> {
    const res = await firstValueFrom(
      this.http.post<ParamAck>(
        '/api/param',
        { syn, value: String(value) },
        { headers: { 'X-Actor': 'web' } },
      ),
    );
    this.snapshot.reload();
    return res;
  }

  /** Toggles server-side write protection, then reloads so the UI reflects it. */
  async setReadOnly(readOnly: boolean): Promise<void> {
    await firstValueFrom(
      this.http.post<{ readOnly: boolean }>(
        '/api/settings',
        { readOnly },
        { headers: { 'X-Actor': 'web' } },
      ),
    );
    this.snapshot.reload();
  }

  // ---- Auswertung (one-off fetches, not polled) ----

  history(channels: string[], from: number, to: number): Promise<HistoryResponse> {
    const params = new HttpParams()
      .set('channels', channels.join(','))
      .set('from', String(from))
      .set('to', String(to));
    return firstValueFrom(this.http.get<HistoryResponse>('/api/history', { params }));
  }

  insights(days: number): Promise<Insights> {
    const params = new HttpParams().set('days', String(days));
    return firstValueFrom(this.http.get<Insights>('/api/insights', { params }));
  }

  // ---- Zeiten (time programs, read-only, fetched on demand) ----

  schedules(): Promise<SchedulesResponse> {
    return firstValueFrom(this.http.get<SchedulesResponse>('/api/schedules'));
  }

  // ---- Heizkurve (heating curve, read-only) ----

  heatCurves(): Promise<HeatCurvesResponse> {
    return firstValueFrom(this.http.get<HeatCurvesResponse>('/api/heatcurve'));
  }

  // ---- Parameter table (par.cgi, read-only) ----

  params(): Promise<ParamsResponse> {
    return firstValueFrom(this.http.get<ParamsResponse>('/api/params'));
  }

  // ---- Audit (change log) ----

  audit(limit = 100): Promise<AuditResponse> {
    const params = new HttpParams().set('limit', String(limit));
    return firstValueFrom(this.http.get<AuditResponse>('/api/audit', { params }));
  }
}
