import DS from "ember-data";
import type ModelRegistry from "ember-data/types/registries/model";
import ApplicationAdapter from "./application";
import RSVP from "rsvp";

export default class GroupAdapter extends ApplicationAdapter {
  /**
   * The Query method for the group model.
   * Returns an array of groups that match the query.
   * Also used by the `queryRecord` method.
   */
  query<K extends keyof ModelRegistry = keyof ModelRegistry>(
    _store: DS.Store,
    _type: ModelRegistry[K],
    query: { query: string }
  ) {
    const results = this.fetchSvc
      .fetch(`/api/${this.configSvc.config.api_version}/groups`, {
        method: "POST",
        body: JSON.stringify({
          // Spaces throw an error, so we replace them with dashes
          query: query.query.replace(" ", "-"),
        }),
      })
      .then((r) => r?.json());

    return RSVP.hash({ results });
  }
}
