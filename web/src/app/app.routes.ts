import { Routes } from '@angular/router';
import { OverviewPage } from './pages/overview';
import { TimeProgramsPage } from './pages/time-programs';
import { HeatCurvePage } from './pages/heat-curve';
import { InsightsPage } from './pages/insights';
import { ParametersPage } from './pages/parameters';
import { LoxonePage } from './pages/loxone';
import { TipsPage } from './pages/tips';
import { LogPage } from './pages/log';
import { SettingsPage } from './pages/settings';

// Route paths are English; the nav labels and page titles stay German (the UI is German).
export const routes: Routes = [
  { path: '', component: OverviewPage, title: 'guntacontrol' },
  { path: 'schedule', component: TimeProgramsPage, title: 'Zeiten · guntacontrol' },
  { path: 'heat-curve', component: HeatCurvePage, title: 'Heizkurve · guntacontrol' },
  { path: 'insights', component: InsightsPage, title: 'Auswertung · guntacontrol' },
  { path: 'tips', component: TipsPage, title: 'Tipps · guntacontrol' },
  { path: 'parameters', component: ParametersPage, title: 'Parameter · guntacontrol' },
  { path: 'loxone', component: LoxonePage, title: 'Loxone · guntacontrol' },
  { path: 'log', component: LogPage, title: 'Protokoll · guntacontrol' },
  { path: 'settings', component: SettingsPage, title: 'Einstellungen · guntacontrol' },
  { path: '**', redirectTo: '' },
];
