// Package featureflags provides feature flag evaluation logic.
package featureflags

import (
	"hash/fnv"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/algolia"
)

//nolint:revive // FeatureFlagsObj name is intentional for clarity in Algolia records
type FeatureFlagsObj struct {
	FeatureFlagUserEmails map[string][]string `json:"featureFlagUserEmails"`
	ObjectID              string              `json:"objectID,omitempty"`
}

// SetAndToggle sets and toggle feature flags.
func SetAndToggle(
	flags *config.FeatureFlags,
	a *algolia.Client,
	h, email string,
	log hclog.Logger) map[string]bool {
	featureFlags := make(map[string]bool)

	// A config with no feature_flags block leaves this nil, which is the
	// common case rather than an error: the caller is the unauthenticated
	// /api/v2/web/config endpoint, and panicking there takes down the one
	// request the frontend must complete before it can do anything at all.
	if flags == nil {
		return featureFlags
	}

	if len(flags.FeatureFlag) > 0 {
		for _, j := range flags.FeatureFlag {
			// Check if "Enabled" is set to enable
			// the feature flag
			switch {
			case j.Enabled != nil:
				// If "Enabled" is set to true,
				// feature flag is set to true.
				// Otherwise, it's set to false.
				featureFlags[j.Name] = *j.Enabled
			case j.Percentage == 0:
				// When percentage is set to 0,
				// the feature flag will remain disabled.
				featureFlags[j.Name] = false
			default:
				// If the percentage is provided in the config
				// the feature flag may be toggled.
				featureFlags[j.Name] = toggleFlagPercentage(
					h,
					j.Percentage,
				)
			}

			// Email based feature flag toggle
			// only when feature flag is set to
			// false. This allows for toggling
			// feature flags for specific
			// users using email address
			if !featureFlags[j.Name] {
				featureFlags[j.Name] = toggleFlagEmail(
					a,
					j.Name,
					email,
					log,
				)
			}
		}
	}

	return featureFlags
}

// toggleFlagPercentage toggles a feature flag
// using an id string and percentage value
// using the built-in hash functions
// This function is based on: https://hashi.co/3O2JwTK
func toggleFlagPercentage(s string, p int) bool {
	h := fnv.New32()
	//nolint:gosec // G104: error from writing to hash is always nil for fnv
	h.Write([]byte(s))
	percent := h.Sum32() % 100
	return int(percent) <= p
}

// toggleFlagEmail toggles a feature flag
// using user email
func toggleFlagEmail(a *algolia.Client, flag, email string, log hclog.Logger) bool {
	// Return false if algolia client is nil (e.g., when using Meilisearch)
	if a == nil {
		return false
	}

	f := FeatureFlagsObj{}
	err := a.Internal.GetObject("featureFlags", &f)
	if err != nil {
		log.Error("error getting featureFlags object from algolia", "error", err)
		return false
	}

	// Enable feature flag if the user email
	// is found in the list of user emails
	// for the feature flag in Algolia
	for _, k := range f.FeatureFlagUserEmails[flag] {
		if strings.EqualFold(email, k) {
			return true
		}
	}

	return false
}
