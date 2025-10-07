import Controller from "@ember/controller";
import { service } from "@ember/service";
import SessionService from "hermes/services/session";
import ConfigService from "hermes/services/config";
import { dropTask } from "ember-concurrency";

export default class AuthenticateController extends Controller {
  @service declare session: SessionService;
  @service("config") declare configSvc: ConfigService;

  protected get currentYear(): number {
    return new Date().getFullYear();
  }

  protected get authProvider(): string {
    return this.configSvc.config.auth_provider || "google";
  }

  protected authenticate = dropTask(async () => {
    if (!this.configSvc.config.skip_google_auth) {
      // Google OAuth flow via Torii.
      await this.session.authenticate(
        "authenticator:torii",
        "google-oauth2-bearer",
      );
      return;
    }

    if (!this.configSvc.config.skip_microsoft_auth) {
      // SharePoint/Microsoft auth is backend-managed. The Go middleware will
      // initiate the Microsoft login flow and handle the callback.
      window.location.href = "/authenticate?init=true";
      return;
    }

    console.error(
      "Microsoft authentication is not properly configured. Missing one of clientId, tenantId, redirectUri.",
    );
  });

  protected authenticateOIDC = dropTask(async () => {
    // For OIDC providers (Okta/Dex), redirect to the backend auth endpoint
    // which will handle the OIDC flow
    const authProvider = this.authProvider;
    window.location.href = `/api/v2/auth/${authProvider}/login`;
  });
}
declare module "@ember/controller" {
  interface Registry {
    authenticate: AuthenticateController;
  }
}
