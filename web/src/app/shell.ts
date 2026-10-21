import { Component, inject } from '@angular/core';
import { DatePipe } from '@angular/common';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { BoilerService } from './core/api';
import { ToastService } from './core/toast';
import { POLL_INTERVAL_MS } from './core/config';

/** App shell: top bar, navigation, status banners, the routed page and the toast. */
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
}
