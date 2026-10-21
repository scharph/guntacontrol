import { Component, inject } from '@angular/core';
import { BoilerService } from '../core/api';
import { ToastService } from '../core/toast';
import { errorText } from '../core/errors';

@Component({
  selector: 'app-settings',
  templateUrl: './settings.html',
  styleUrl: './settings.scss',
})
export class SettingsPage {
  protected readonly boiler = inject(BoilerService);
  private readonly toast = inject(ToastService);

  /**
   * Toggles write protection. Like the Heliotherm app, switching it OFF asks for
   * confirmation first, since it then allows changes to the boiler.
   */
  protected async toggleReadOnly(): Promise<void> {
    const currentlyProtected = this.boiler.readOnly();
    if (currentlyProtected) {
      const ok = confirm(
        'Schreibschutz ausschalten? Danach können Einstellungen am Kessel geändert werden.',
      );
      if (!ok) return;
    }
    try {
      await this.boiler.setReadOnly(!currentlyProtected);
      this.toast.show('ok', currentlyProtected ? 'Schreibschutz aus' : 'Schreibschutz ein');
    } catch (e) {
      this.toast.show('err', errorText(e));
    }
  }
}
