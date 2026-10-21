import { Component, effect, inject, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { BoilerService } from './core/api';
import { ToastService } from './core/toast';
import { POLL_INTERVAL_MS } from './core/config';

type Theme = 'auto' | 'light' | 'dark';

const NAV: { path: string; label: string; icon: string; exact?: boolean }[] = [
  { path: '/', label: 'Übersicht', icon: '◉', exact: true },
  { path: '/zeiten', label: 'Zeiten', icon: '🕑' },
  { path: '/auswertung', label: 'Auswertung', icon: '📈' },
  { path: '/loxone', label: 'Loxone', icon: '🏠' },
  { path: '/protokoll', label: 'Protokoll', icon: '📝' },
  { path: '/einstellungen', label: 'Einstellungen', icon: '⚙' },
];

/** App shell: sidebar navigation, sticky status header, the routed page and the toast. */
@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, DatePipe],
  templateUrl: './shell.html',
  styleUrl: './shell.scss',
})
export class Shell {
  protected readonly boiler = inject(BoilerService);
  protected readonly toasts = inject(ToastService);
  protected readonly pollSeconds = POLL_INTERVAL_MS / 1000;
  protected readonly nav = NAV;

  protected readonly navOpen = signal(false);
  protected readonly theme = signal<Theme>(readStoredTheme());

  constructor() {
    // Apply the chosen theme to <html>: 'auto' follows the OS (no attribute), the
    // others pin data-theme. Persisted in localStorage so it survives reloads.
    effect(() => {
      const t = this.theme();
      const root = document.documentElement;
      if (t === 'auto') root.removeAttribute('data-theme');
      else root.setAttribute('data-theme', t);
      try {
        localStorage.setItem('gc-theme', t);
      } catch {
        /* private mode / blocked storage: theme just won't persist */
      }
    });
  }

  protected cycleTheme(): void {
    const order: Theme[] = ['auto', 'light', 'dark'];
    this.theme.update((t) => order[(order.indexOf(t) + 1) % order.length]);
  }

  protected themeIcon(): string {
    return { auto: '◐', light: '☀', dark: '☾' }[this.theme()];
  }
}

function readStoredTheme(): Theme {
  try {
    const t = localStorage.getItem('gc-theme');
    if (t === 'light' || t === 'dark' || t === 'auto') return t;
  } catch {
    /* ignore */
  }
  return 'auto';
}
