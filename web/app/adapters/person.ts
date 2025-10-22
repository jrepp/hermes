import DS from "ember-data";
import type ModelRegistry from "ember-data/types/registries/model";
import ApplicationAdapter from "./application";
import RSVP from "rsvp";

export default class PersonAdapter extends ApplicationAdapter {
  /**
   * Queries using the `body` parameter instead of a queryParam.
   * Default query:     `/people?query=foo`
   * Our custom query:  `/people` with `{ query: "foo" }` in the request body.
   */
  query<K extends keyof ModelRegistry = keyof ModelRegistry>(
    _store: DS.Store,
    _type: ModelRegistry[K],
    query: { query: string }
  ) {
    console.log('[PersonAdapter] 🌐 query() called', { query: query.query, apiVersion: this.configSvc.config.api_version });
    
    const results = this.fetchSvc
      .fetch(`/api/${this.configSvc.config.api_version}/people`, {
        method: "POST",
        body: JSON.stringify({
          query: query.query,
        }),
      })
      .then((r) => {
        console.log('[PersonAdapter] 📡 Fetch response received', { status: r?.status, ok: r?.ok });
        return r?.json();
      })
      .then((data) => {
        console.log('[PersonAdapter] 📦 JSON parsed', { dataLength: data?.length, data });
        return data;
      })
      .catch((error) => {
        console.error('[PersonAdapter] ❌ Fetch error:', error);
        throw error;
      });

    const wrappedResults = RSVP.hash({ results });
    console.log('[PersonAdapter] 📮 Returning wrapped results promise');
    return wrappedResults;
  }

  /**
   * Queries for a single person record using emailAddress parameter.
   * Used by store.queryRecord("person", { emails: "user@example.com" })
   */
  queryRecord<K extends keyof ModelRegistry = keyof ModelRegistry>(
    _store: DS.Store,
    _type: ModelRegistry[K],
    query: { emails: string }
  ) {
    return RSVP.Promise.resolve(
      this.fetchSvc
        .fetch(`/api/${this.configSvc.config.api_version}/people?emailAddress=${query.emails}`)
        .then((r) => r?.json())
    );
  }
}
