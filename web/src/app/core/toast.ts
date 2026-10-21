import { Injectable, signal } from '@angular/core';

export interface Toast {
  kind: 'ok' | 'err';
  text: string;
}

/** Shared transient toast state, rendered once by the shell. */
@Injectable({ providedIn: 'root' })
export class ToastService {
  readonly toast = signal<Toast | null>(null);

  show(kind: 'ok' | 'err', text: string): void {
    this.toast.set({ kind, text });
    setTimeout(() => this.toast.set(null), 4000);
  }

  clear(): void {
    this.toast.set(null);
  }
}
