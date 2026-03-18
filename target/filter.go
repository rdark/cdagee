package target

// FilterByTags returns targets that have at least one of the listed tags
// (OR semantics). If tags is empty, all targets are returned unchanged.
//
// DependsOn entries referencing targets outside the filtered set are
// stripped so that the result can be passed to BuildGraph without dangling
// reference errors. The input slice is not mutated; returned Target
// values are shallow copies with a new DependsOn slice where needed.
func FilterByTags(targets []Target, tags []string) []Target {
	if len(tags) == 0 {
		return targets
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}

	// First pass: collect matching target IDs.
	kept := make(map[string]struct{})
	var filtered []Target
	for _, tgt := range targets {
		if matchesAny(tgt.Config.Tags, tagSet) {
			kept[tgt.ID] = struct{}{}
			filtered = append(filtered, tgt)
		}
	}

	// Second pass: strip depends_on entries that reference filtered-out targets.
	for i, tgt := range filtered {
		var pruned []string
		for _, dep := range tgt.Config.DependsOn {
			if _, ok := kept[dep]; ok {
				pruned = append(pruned, dep)
			}
		}
		if len(pruned) != len(tgt.Config.DependsOn) {
			filtered[i].Config.DependsOn = pruned
		}
	}

	return filtered
}

func matchesAny(tgtTags []string, want map[string]struct{}) bool {
	for _, t := range tgtTags {
		if _, ok := want[t]; ok {
			return true
		}
	}
	return false
}
