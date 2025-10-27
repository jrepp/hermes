import Route from '@ember/routing/route';
import { service } from '@ember/service';
import type RouterService from '@ember/routing/router-service';
import type ConfigService from 'hermes/services/config';

interface SetupStatusResponse {
  is_configured: boolean;
  config_path?: string;
  working_dir: string;
}

export default class SetupRoute extends Route {
  @service declare router: RouterService;
  @service declare config: ConfigService;

  async model() {
    try {
      const response = await fetch('/api/v2/setup/status');
      const status: SetupStatusResponse = await response.json();

      // If already configured, redirect to home
      if (status.is_configured) {
        this.router.transitionTo('authenticated');
        return null;
      }

      return {
        workingDir: status.working_dir,
        workspacePath: 'docs-cms', // Default value
        upstreamURL: '',
      };
    } catch (error) {
      console.error('Error checking setup status:', error);
      return {
        workingDir: '',
        workspacePath: 'docs-cms',
        upstreamURL: '',
      };
    }
  }
}
