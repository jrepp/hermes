import Route from "@ember/routing/route";
import RouterService from "@ember/routing/router-service";
import type Transition from "@ember/routing/transition";
import { service } from "@ember/service";

export default class AuthenticatedDraftsRoute extends Route {
  @service declare router: RouterService;

  beforeModel(_transition: Transition) {
    void this.router.transitionTo("authenticated.my");
  }
}
