import Route from "@ember/routing/route";
import { service } from "@ember/service";
import ConfigService from "hermes/services/config";
import RouterService from "@ember/routing/router-service";
import SessionService from "hermes/services/session";

export default class AuthenticateRoute extends Route {
  @service("config") declare configSvc: ConfigService;
  @service declare router: RouterService;
  @service declare session: SessionService;

  async beforeModel() {
    /**
     * Checks if the session is authenticated,
     * and if it is, transitions to the specified route.
     * If it's not, the route will render normally.
     */
    this.session.prohibitAuthentication("/");
  }

  async model() {
    // In SharePoint mode, authentication is backend-managed via secure cookies.
    // If a valid backend session exists, establish the frontend session and
    // continue to the app.
    if (
      this.configSvc.config.skip_google_auth &&
      !this.configSvc.config.skip_microsoft_auth
    ) {
      try {
        await this.session.authenticate("authenticator:cookie");
        this.router.replaceWith("/");
        return;
      } catch (error) {
        // No backend session yet. Allow the route to render normally.
      }
    }
  }
}
