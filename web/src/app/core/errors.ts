import { HttpErrorResponse } from '@angular/common/http';

/** Human-readable German message for a failed HTTP call to the backend. */
export function errorText(e: unknown): string {
  if (e instanceof HttpErrorResponse) {
    const body = e.error;
    if (body && typeof body === 'object' && 'error' in body) {
      return String((body as { error: unknown }).error);
    }
    if (e.status === 0) return 'Server nicht erreichbar';
    return `HTTP ${e.status}`;
  }
  return 'Unbekannter Fehler';
}
