import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideHttpClient, withFetch } from '@angular/common/http';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    // Angular 22 runs zoneless by default; HttpClient (with the Fetch backend)
    // powers both httpResource reads and the control POSTs.
    provideHttpClient(withFetch()),
  ],
};
