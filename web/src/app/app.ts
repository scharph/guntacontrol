import { Component } from '@angular/core';
import { Shell } from './shell';

// Root component: the whole UI lives in the routed shell.
@Component({
  selector: 'app-root',
  imports: [Shell],
  template: '<app-shell />',
})
export class App {}
