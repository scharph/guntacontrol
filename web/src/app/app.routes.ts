import { Routes } from '@angular/router';
import { OverviewPage } from './pages/overview';
import { TimeProgramsPage } from './pages/time-programs';
import { HeatCurvePage } from './pages/heat-curve';
import { InsightsPage } from './pages/insights';
import { LoxonePage } from './pages/loxone';
import { LogPage } from './pages/log';
import { SettingsPage } from './pages/settings';

export const routes: Routes = [
  { path: '', component: OverviewPage, title: 'guntacontrol' },
  { path: 'zeiten', component: TimeProgramsPage, title: 'Zeiten · guntacontrol' },
  { path: 'heizkurve', component: HeatCurvePage, title: 'Heizkurve · guntacontrol' },
  { path: 'auswertung', component: InsightsPage, title: 'Auswertung · guntacontrol' },
  { path: 'loxone', component: LoxonePage, title: 'Loxone · guntacontrol' },
  { path: 'protokoll', component: LogPage, title: 'Protokoll · guntacontrol' },
  { path: 'einstellungen', component: SettingsPage, title: 'Einstellungen · guntacontrol' },
  { path: '**', redirectTo: '' },
];
