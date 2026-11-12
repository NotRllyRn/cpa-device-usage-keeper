package service

import (
	"cpa-usage-keeper/internal/pricingmetadata"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"strings"
)

// MatchCompletePricingCatalog reuses Keeper's candidate ordering and price
// conversion, but requires an exact catalog identity and all four rates. Daily
// aggregates cannot safely infer a missing cache price or a model family alias.
func MatchCompletePricingCatalog(models []string, catalog pricingmetadata.Catalog) servicedto.PricingSyncFetchResponse {
	out := servicedto.PricingSyncFetchResponse{Source: catalog.Source.ID, Matches: []servicedto.PricingSyncFetchMatch{}, UnmatchedModels: []string{}}
	seen := map[string]bool{}
	for _, model := range models {
		if seen[model] {
			continue
		}
		seen[model] = true
		name := strings.TrimPrefix(model, "[pi] ")
		candidates := pricingmetadata.Catalog{Source: catalog.Source}
		for _, entry := range catalog.Entries {
			cost := entry.Model.Cost
			if cost.Input == nil || cost.Output == nil || cost.CacheRead == nil || cost.CacheWrite == nil {
				continue
			}
			if name == entry.Model.ID || name == entry.Model.Name || name == entry.ProviderID+"/"+entry.Model.ID {
				candidates.Entries = append(candidates.Entries, entry)
			}
		}
		matches, _ := collectPricingSyncMatches([]string{name}, candidates)
		if len(matches) == 0 {
			out.UnmatchedModels = append(out.UnmatchedModels, model)
			continue
		}
		match := matches[0]
		out.Matches = append(out.Matches, servicedto.PricingSyncFetchMatch{Model: model, MatchedModel: match.MatchedModel, Provider: match.SourceProviderName, PricingStyle: match.PricingStyle, BasePrices: match.BasePrices})
	}
	return out
}
