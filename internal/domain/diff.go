package domain

import "sort"

type ChangeDelta struct {
	Locator       string       `json:"locator"`
	Kind          string       `json:"kind"`
	Previous      *ChangeBlock `json:"previous,omitempty"`
	Current       *ChangeBlock `json:"current,omitempty"`
	ChangedFields []string     `json:"changedFields,omitempty"`
}
type RevisionDiff struct {
	FromRevision int           `json:"fromRevision"`
	ToRevision   int           `json:"toRevision"`
	Deltas       []ChangeDelta `json:"deltas"`
}

func (c *RevisionCase) Diff(from, to int) RevisionDiff {
	result := RevisionDiff{FromRevision: from, ToRevision: to, Deltas: []ChangeDelta{}}
	oldBlocks := blocksByLocator(c.Blocks, from)
	newBlocks := blocksByLocator(c.Blocks, to)
	keys := make([]string, 0, len(oldBlocks)+len(newBlocks))
	seen := map[string]bool{}
	for key := range oldBlocks {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range newBlocks {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		oldBlock, hadOld := oldBlocks[key]
		newBlock, hasNew := newBlocks[key]
		switch {
		case !hadOld:
			copyNew := newBlock
			result.Deltas = append(result.Deltas, ChangeDelta{Locator: key, Kind: "added", Current: &copyNew})
		case !hasNew:
			copyOld := oldBlock
			result.Deltas = append(result.Deltas, ChangeDelta{Locator: key, Kind: "removed", Previous: &copyOld})
		default:
			fields := changedFields(oldBlock, newBlock)
			if len(fields) > 0 {
				copyOld, copyNew := oldBlock, newBlock
				result.Deltas = append(result.Deltas, ChangeDelta{Locator: key, Kind: "modified", Previous: &copyOld, Current: &copyNew, ChangedFields: fields})
			}
		}
	}
	return result
}
func blocksByLocator(blocks []ChangeBlock, revision int) map[string]ChangeBlock {
	out := map[string]ChangeBlock{}
	for _, b := range blocks {
		if b.RevisionIndex == revision {
			out[blockLocator(b)] = b
		}
	}
	return out
}
func blockLocator(b ChangeBlock) string {
	return b.Chapter + "/" + b.TaskNumber + "/" + b.SourceLocator
}
func changedFields(a, b ChangeBlock) []string {
	fields := []string{}
	if a.ReplacementText != b.ReplacementText {
		fields = append(fields, "replacementText")
	}
	if a.WarningText != b.WarningText {
		fields = append(fields, "warningText")
	}
	if a.AffectedProcedure != b.AffectedProcedure {
		fields = append(fields, "affectedProcedure")
	}
	if a.EngineeringReference != b.EngineeringReference {
		fields = append(fields, "engineeringReference")
	}
	if a.ApprovalReference != b.ApprovalReference {
		fields = append(fields, "approvalReference")
	}
	if a.ConfigurationScope != b.ConfigurationScope {
		fields = append(fields, "configurationScope")
	}
	return fields
}
