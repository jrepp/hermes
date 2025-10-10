import Component from "@glimmer/component";
import { tracked } from "@glimmer/tracking";
import { service } from "@ember/service";
import { restartableTask, timeout } from "ember-concurrency";
import { action } from "@ember/object";
import ConfigService from "hermes/services/config";
import FetchService from "hermes/services/fetch";
import { isTesting } from "@embroider/macros";
import StoreService from "hermes/services/store";
import PersonModel from "hermes/models/person";
import { Select } from "ember-power-select/components/power-select";
import { next, schedule } from "@ember/runloop";
import calculatePosition from "ember-basic-dropdown/utils/calculate-position";
import GroupModel from "hermes/models/group";
import AuthenticatedUserService from "hermes/services/authenticated-user";

export interface GoogleUser {
  emailAddresses: { value: string }[];
  names: { displayName: string; givenName: string }[];
  photos: { url: string }[];
}

enum ComputedVerticalPosition {
  Above = "above",
  Below = "below",
}

enum ComputedHorizontalPosition {
  Left = "left",
  Right = "right",
}

interface CalculatePositionOptions {
  horizontalPosition: ComputedHorizontalPosition;
  verticalPosition: ComputedVerticalPosition;
  matchTriggerWidth: boolean;
  previousHorizontalPosition?: ComputedHorizontalPosition;
  previousVerticalPosition?: ComputedVerticalPosition;
  renderInPlace: boolean;
  dropdown: any;
}

interface InputsPeopleSelectComponentSignature {
  Element: HTMLDivElement;
  Args: {
    selected: string[];
    onChange: (value: string[]) => void;
    renderInPlace?: boolean;
    disabled?: boolean;
    includeGroups?: boolean;
    isSingleSelect?: boolean;
    excludeSelf?: boolean;
    triggerId?: string;
  };
}

const DEBOUNCE_MS = Ember.testing ? 0 : 300;
const MAX_RETRIES = 3;
const INITIAL_RETRY_DELAY = isTesting() ? 0 : 500;

export default class InputsPeopleSelectComponent extends Component<InputsPeopleSelectComponentSignature> {
  @service("config") declare configSvc: ConfigService;
  @service declare authenticatedUser: AuthenticatedUserService;
  @service declare store: StoreService;

  @tracked searchQuery = "";
  @tracked searchResults: string[] = [];
  @tracked showDropdown = false;
  @tracked isSearching = false;
  @tracked focusedIndex = -1;
  @tracked dropdownElement: HTMLElement | null = null;

  @action
  setupDropdown(element: HTMLElement) {
    this.dropdownElement = element;
  }

  @action
  onSearchInput(event: Event) {
    const input = event.target as HTMLInputElement;
    this.searchQuery = input.value;
    
    if (this.searchQuery.trim().length > 0) {
      this.showDropdown = true;
      debounce(this, this.performSearch, DEBOUNCE_MS);
    } else {
      this.showDropdown = false;
      this.searchResults = [];
    }
  }

  /**
   * A task that queries the server for people matching the given query.
   * Used as the `search` action for the `ember-power-select` component.
   * Sets `this.people` to the results of the query.
   * Includes a 250ms debounce to prevent rapid cancellations from quick typing.
   */
  protected searchDirectory = restartableTask(async (query: string) => {
    console.log('[PeopleSelect] 🔍 searchDirectory task started', { query, includeGroups: this.args.includeGroups, hasStore: !!this.store });
    
    // Debounce: wait 250ms before starting the search to prevent rapid cancellations
    console.log('[PeopleSelect] ⏱️ Debouncing for 250ms...');
    await timeout(250);
    console.log('[PeopleSelect] ✅ Debounce complete, proceeding with search');
    
    if (!this.store) {
      console.error('[PeopleSelect] ❌ ERROR: this.store is undefined!');
      return;
    }
    
    for (let i = 0; i < MAX_RETRIES; i++) {
      let retryDelay = INITIAL_RETRY_DELAY;

      let p: string[] = [];
      let g: string[] = [];

      /**
       * WORKAROUND: Directly call adapter methods instead of Store.query()
       * to avoid Ember Data 4.12 RequestManager initialization issues.
       * See web/app/services/_store.ts maybeFetchPeople for same workaround.
       */
      const personAdapter = this.store.adapterFor("person" as never) as any;
      const personSerializer = this.store.serializerFor("person" as never) as any;
      
      const promises = [
        personAdapter.query(this.store, this.store.modelFor("person"), { query }),
      ];

      try {
        let promises: Promise<any>[] = [this.store.query("person", { query })];

        if (this.args.includeGroups) {
          const groupAdapter = this.store.adapterFor("group" as never) as any;
          promises.push(groupAdapter.query(this.store, this.store.modelFor("group"), { query }));
        }

        const [peopleResponse, groupsResponse] = await Promise.all(promises);
        
        // Push person records through serializer to add them to the store
        if (peopleResponse?.results && peopleResponse.results.length > 0) {
          console.log('[PeopleSelect] 📥 Processing person results', { count: peopleResponse.results.length, results: peopleResponse.results });
          peopleResponse.results.forEach((personData: any) => {
            const email = personData.emailAddresses?.[0]?.value;
            console.log('[PeopleSelect] 👤 Processing person data', { email, personData });
            if (email) {
              // Create or update the person record in the store
              const pushData = {
                data: {
                  id: email,
                  type: "person",
                  attributes: {
                    name: personData.names?.[0]?.displayName || "",
                    firstName: personData.names?.[0]?.givenName || "",
                    email: email,
                    picture: personData.photos?.[0]?.url || "",
                  },
                },
              };
              console.log('[PeopleSelect] 💾 Pushing to store', { pushData });
              this.store.push(pushData);
              
              // Verify the record was added
              const record = this.store.peekRecord("person", email);
              console.log('[PeopleSelect] 🔍 Peek after push', { email, record, recordExists: !!record, recordData: record ? { name: record.get('name'), email: record.get('email') } : null });
            }
          });
        }
        
        // Push group records through serializer if groups are included
        if (groupsResponse?.results && groupsResponse.results.length > 0) {
          groupsResponse.results.forEach((groupData: any) => {
            const email = groupData.email;
            if (email) {
              // Create or update the group record in the store
              this.store.push({
                data: {
                  id: email,
                  type: "group",
                  attributes: {
                    name: groupData.name || "",
                    email: email,
                  },
                },
              });
            }
          });
        }
        
        // Unwrap adapter responses (adapters return { results: [...] })
        const people = peopleResponse?.results;
        const groups = groupsResponse?.results;

        if (people) {
          p = people
            .map((p: any) => p.emailAddresses?.[0]?.value)
            .filter((email: string | undefined) => !!email) // Filter out undefined emails
            .filter((email: string) => {
              // Filter out already selected
              return !this.args.selected.includes(email);
            })
            .filter((email: string) => {
              // filter the authenticated user if `excludeSelf` is true
              return (
                !this.args.excludeSelf ||
                email !== this.authenticatedUser.info?.email
              );
            });
        }

        if (groups) {
          g = groups
            .filter((group: GroupModel) => {
              const name = group.name.toLowerCase();
              return !name.includes("departed") && !name.includes("terminated");
            })
            .map((g: GroupModel) => g.email)
            .filter((email: string) => {
              // Filter out any people already selected
              return !this.args.selected.find(
                (selectedEmail) => selectedEmail === email,
              );
            });
        }

        // Concatenate and sort alphabetically
        this.options = [...p, ...g].sort((a, b) => a.localeCompare(b));
        console.log('[PeopleSelect] ✅ Search complete', { totalResults: this.options.length, people: p.length, groups: g.length, options: this.options });
        
        // Verify all options can be found in store
        this.options.forEach((email) => {
          const personRecord = this.store.peekRecord("person", email);
          const groupRecord = this.store.peekRecord("group", email);
          console.log('[PeopleSelect] 🔎 Option verification', { email, personRecord: !!personRecord, groupRecord: !!groupRecord, personName: personRecord?.get('name'), groupName: groupRecord?.get('name') });
        });

        // Stop the loop if the query was successful
        return;
      } catch (e) {
        console.error('[PeopleSelect] ❌ Error on attempt', i + 1, ':', e);
        // Throw an error if this is the last retry.
        if (i === MAX_RETRIES - 1) {
          console.error(`[PeopleSelect] 💥 All retries exhausted. Final error: ${e}`);
          throw e;
        } else {
          console.log('[PeopleSelect] 🔄 Retrying after', retryDelay, 'ms delay...');
        }

        // Wait and retry
        await new Promise(resolve => setTimeout(resolve, retryDelay));
        retryDelay *= 2;
      }
    }
    console.log('[PeopleSelect] 🏁 searchDirectory task completed');
  });
}

declare module "@glint/environment-ember-loose/registry" {
  export default interface Registry {
    "Inputs::PeopleSelect": typeof InputsPeopleSelectComponent;
  }
}