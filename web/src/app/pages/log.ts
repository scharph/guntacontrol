import { Component, inject, signal } from '@angular/core';
import { BoilerService } from '../core/api';
import { AuditEntry } from '../core/models';

@Component({
  selector: 'app-log',
  templateUrl: './log.html',
  styleUrl: './log.scss',
})
export class LogPage {
  private readonly boiler = inject(BoilerService);

  protected readonly entries = signal<AuditEntry[]>([]);
  protected readonly loading = signal(false);
  protected readonly error = signal('');

  constructor() {
    this.reload();
  }

  protected async reload(): Promise<void> {
    this.loading.set(true);
    try {
      const res = await this.boiler.audit(200);
      this.entries.set(res.entries);
      this.error.set('');
    } catch {
      this.error.set('Protokoll konnte nicht geladen werden.');
    } finally {
      this.loading.set(false);
    }
  }

  protected when(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleString('de-DE', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }
}
