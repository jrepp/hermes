import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { service } from '@ember/service';
import type RouterService from '@ember/routing/router-service';

interface SetupArgs {
  workingDir: string;
  workspacePath: string;
  upstreamURL: string;
}

interface SetupResponse {
  success: boolean;
  config_path: string;
  workspace_dir: string;
  message: string;
}

interface OllamaValidationResponse {
  valid: boolean;
  message: string;
  version?: string;
}

export default class SetupWizardComponent extends Component<{ Args: SetupArgs }> {
  @service declare router: RouterService;

  @tracked workspacePath = this.args.workspacePath || 'docs-cms';
  @tracked upstreamURL = this.args.upstreamURL || '';
  @tracked ollamaURL = 'http://localhost:11434';
  @tracked ollamaModel = 'llama3.2';
  @tracked isSubmitting = false;
  @tracked errorMessage = '';
  @tracked successMessage = '';
  @tracked isValidatingOllama = false;
  @tracked ollamaValidationMessage = '';
  @tracked ollamaValidationSuccess = false;

  get workingDirDisplay() {
    return this.args.workingDir || 'Loading...';
  }

  get fullWorkspacePath() {
    if (!this.workspacePath) {
      return this.workingDirDisplay;
    }
    return `${this.workingDirDisplay}/${this.workspacePath}`;
  }

  @action
  updateWorkspacePath(event: Event) {
    const target = event.target as HTMLInputElement;
    this.workspacePath = target.value;
    this.errorMessage = '';
  }

  @action
  updateUpstreamURL(event: Event) {
    const target = event.target as HTMLInputElement;
    this.upstreamURL = target.value;
    this.errorMessage = '';
  }

  @action
  updateOllamaURL(event: Event) {
    const target = event.target as HTMLInputElement;
    this.ollamaURL = target.value;
    this.errorMessage = '';
    this.ollamaValidationMessage = '';
  }

  @action
  updateOllamaModel(event: Event) {
    const target = event.target as HTMLInputElement;
    this.ollamaModel = target.value;
    this.errorMessage = '';
    this.ollamaValidationMessage = '';
  }

  @action
  async validateOllama(event: Event) {
    event.preventDefault();
    
    this.isValidatingOllama = true;
    this.ollamaValidationMessage = '';
    this.ollamaValidationSuccess = false;

    try {
      const response = await fetch('/api/v2/setup/validate-ollama', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url: this.ollamaURL,
          model: this.ollamaModel,
        }),
      });

      if (!response.ok) {
        throw new Error('Validation request failed');
      }

      const result: OllamaValidationResponse = await response.json();
      this.ollamaValidationMessage = result.message;
      this.ollamaValidationSuccess = result.valid;
    } catch (error) {
      console.error('Ollama validation error:', error);
      this.ollamaValidationMessage = 'Could not validate Ollama connection';
      this.ollamaValidationSuccess = false;
    } finally {
      this.isValidatingOllama = false;
    }
  }

  @action
  async submitSetup(event: Event) {
    event.preventDefault();
    
    this.isSubmitting = true;
    this.errorMessage = '';
    this.successMessage = '';

    try {
      const response = await fetch('/api/v2/setup/configure', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          workspace_path: this.workspacePath,
          upstream_url: this.upstreamURL,
          ollama_url: this.ollamaURL,
          ollama_model: this.ollamaModel,
        }),
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Configuration failed');
      }

      const result: SetupResponse = await response.json();

      if (result.success) {
        this.successMessage = result.message;
        
        // Wait a moment then reload to pick up new config
        setTimeout(() => {
          window.location.href = '/';
        }, 2000);
      } else {
        this.errorMessage = 'Configuration failed. Please try again.';
      }
    } catch (error) {
      console.error('Setup error:', error);
      this.errorMessage = error instanceof Error ? error.message : 'An error occurred during setup';
    } finally {
      this.isSubmitting = false;
    }
  }
}
