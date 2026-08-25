package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type ChangeInput struct{ ID, Chapter, TaskNumber, SourceLocator, ReplacementText, WarningText, AffectedProcedure, EngineeringReference, ApprovalReference, ConfigurationScope string }

type ChangeOrder struct {
	ID       string `json:"id"`
	Sequence int    `json:"sequence"`
}

func (c *RevisionCase) AddChange(in ChangeInput) error {
	if err := c.AssertMutable(); err != nil {
		return err
	}
	if c.Status != StatusDraft && c.Status != StatusRemediation {
		return ErrInvalidState
	}
	if err := validateChangeInput(in, true); err != nil {
		return err
	}
	for _, b := range c.Blocks {
		if b.RevisionIndex == c.CurrentRevision && locatorKey(b.Chapter, b.TaskNumber, b.SourceLocator) == locatorKey(in.Chapter, in.TaskNumber, in.SourceLocator) {
			return ErrDuplicateLocator
		}
		if b.ID == in.ID {
			return Violation{Field: "id", Message: "变更块 ID 已存在"}
		}
	}
	b := ChangeBlock{ID: strings.TrimSpace(in.ID), CaseID: c.ID, RevisionIndex: c.CurrentRevision, Sequence: c.nextSequence(), Chapter: strings.TrimSpace(in.Chapter), TaskNumber: strings.TrimSpace(in.TaskNumber), SourceLocator: strings.TrimSpace(in.SourceLocator), ReplacementText: strings.TrimSpace(in.ReplacementText), WarningText: strings.TrimSpace(in.WarningText), AffectedProcedure: strings.TrimSpace(in.AffectedProcedure), EngineeringReference: strings.TrimSpace(in.EngineeringReference), ApprovalReference: strings.TrimSpace(in.ApprovalReference), ConfigurationScope: strings.TrimSpace(in.ConfigurationScope)}
	c.Blocks = append(c.Blocks, b)
	c.InvalidateChecks()
	for i := range c.Rounds {
		if c.Rounds[i].Index == c.CurrentRevision {
			c.Rounds[i].BlockIDs = append(c.Rounds[i].BlockIDs, b.ID)
		}
	}
	return nil
}

func (c *RevisionCase) UpdateChange(id string, in ChangeInput) error {
	if err := c.assertCurrentRoundEditable(); err != nil {
		return err
	}
	if err := validateChangeInput(in, false); err != nil {
		return err
	}
	index := -1
	for i := range c.Blocks {
		b := c.Blocks[i]
		if b.ID == id {
			if b.RevisionIndex != c.CurrentRevision {
				return ErrNotFound
			}
			index = i
			continue
		}
		if b.RevisionIndex == c.CurrentRevision && locatorKey(b.Chapter, b.TaskNumber, b.SourceLocator) == locatorKey(in.Chapter, in.TaskNumber, in.SourceLocator) {
			return ErrDuplicateLocator
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	b := &c.Blocks[index]
	b.Chapter = strings.TrimSpace(in.Chapter)
	b.TaskNumber = strings.TrimSpace(in.TaskNumber)
	b.SourceLocator = strings.TrimSpace(in.SourceLocator)
	b.ReplacementText = strings.TrimSpace(in.ReplacementText)
	b.WarningText = strings.TrimSpace(in.WarningText)
	b.AffectedProcedure = strings.TrimSpace(in.AffectedProcedure)
	b.EngineeringReference = strings.TrimSpace(in.EngineeringReference)
	b.ApprovalReference = strings.TrimSpace(in.ApprovalReference)
	b.ConfigurationScope = strings.TrimSpace(in.ConfigurationScope)
	c.InvalidateChecks()
	return nil
}

func (c *RevisionCase) DeleteChange(id string) error {
	if err := c.assertCurrentRoundEditable(); err != nil {
		return err
	}
	index := -1
	for i, b := range c.Blocks {
		if b.ID == id && b.RevisionIndex == c.CurrentRevision {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	c.Blocks = append(c.Blocks[:index], c.Blocks[index+1:]...)
	for i := range c.Rounds {
		if c.Rounds[i].Index != c.CurrentRevision {
			continue
		}
		ids := c.Rounds[i].BlockIDs[:0]
		for _, blockID := range c.Rounds[i].BlockIDs {
			if blockID != id {
				ids = append(ids, blockID)
			}
		}
		c.Rounds[i].BlockIDs = ids
	}
	c.resequenceCurrent(c.CurrentBlocks())
	c.syncCurrentRoundBlockIDs()
	c.InvalidateChecks()
	return nil
}

func (c *RevisionCase) ReorderChanges(order []ChangeOrder) error {
	if err := c.assertCurrentRoundEditable(); err != nil {
		return err
	}
	blocks := c.CurrentBlocks()
	if len(order) != len(blocks) {
		return ErrInvalidOrder
	}
	current := make(map[string]bool, len(blocks))
	for _, b := range blocks {
		current[b.ID] = true
	}
	seenIDs, seenSequences := map[string]bool{}, map[int]bool{}
	for _, item := range order {
		id := strings.TrimSpace(item.ID)
		if !current[id] || seenIDs[id] || item.Sequence < 1 || item.Sequence > len(order) || seenSequences[item.Sequence] {
			return ErrInvalidOrder
		}
		seenIDs[id], seenSequences[item.Sequence] = true, true
	}
	for i := 1; i <= len(order); i++ {
		if !seenSequences[i] {
			return ErrInvalidOrder
		}
	}
	sequenceByID := map[string]int{}
	for _, item := range order {
		sequenceByID[strings.TrimSpace(item.ID)] = item.Sequence
	}
	for i := range c.Blocks {
		if c.Blocks[i].RevisionIndex == c.CurrentRevision {
			c.Blocks[i].Sequence = sequenceByID[c.Blocks[i].ID]
		}
	}
	c.syncCurrentRoundBlockIDs()
	c.InvalidateChecks()
	return nil
}

func (c *RevisionCase) assertCurrentRoundEditable() error {
	if err := c.AssertMutable(); err != nil {
		return err
	}
	if c.Status != StatusDraft && c.Status != StatusRemediation {
		return ErrInvalidState
	}
	return nil
}

func validateChangeInput(in ChangeInput, requireID bool) error {
	fields := []struct{ name, value string }{{"chapter", in.Chapter}, {"taskNumber", in.TaskNumber}, {"sourceLocator", in.SourceLocator}, {"replacementText", in.ReplacementText}, {"affectedProcedure", in.AffectedProcedure}}
	if requireID {
		fields = append([]struct{ name, value string }{{"id", in.ID}}, fields...)
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return Violation{Field: field.name, Message: "关键变更内容不能为空"}
		}
	}
	return nil
}

func locatorKey(chapter, task, locator string) string {
	normalize := func(v string) string { return strings.ToUpper(strings.Join(strings.Fields(v), " ")) }
	return normalize(chapter) + "\x1f" + normalize(task) + "\x1f" + normalize(locator)
}

func (c *RevisionCase) resequenceCurrent(blocks []ChangeBlock) {
	sequences := map[string]int{}
	for index, b := range blocks {
		sequences[b.ID] = index + 1
	}
	for i := range c.Blocks {
		if c.Blocks[i].RevisionIndex == c.CurrentRevision {
			c.Blocks[i].Sequence = sequences[c.Blocks[i].ID]
		}
	}
}

func (c *RevisionCase) syncCurrentRoundBlockIDs() {
	blocks := c.CurrentBlocks()
	ids := make([]string, 0, len(blocks))
	for _, block := range blocks {
		ids = append(ids, block.ID)
	}
	for i := range c.Rounds {
		if c.Rounds[i].Index == c.CurrentRevision {
			c.Rounds[i].BlockIDs = ids
			return
		}
	}
}
func (c *RevisionCase) nextSequence() int {
	max := 0
	for _, b := range c.Blocks {
		if b.RevisionIndex == c.CurrentRevision && b.Sequence > max {
			max = b.Sequence
		}
	}
	return max + 1
}
func (c *RevisionCase) CurrentBlocks() []ChangeBlock {
	result := []ChangeBlock{}
	for _, b := range c.Blocks {
		if b.RevisionIndex == c.CurrentRevision {
			result = append(result, b)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Sequence == result[j].Sequence {
			return result[i].ID < result[j].ID
		}
		return result[i].Sequence < result[j].Sequence
	})
	return result
}
func FindBlock(c *RevisionCase, id string) (ChangeBlock, bool) {
	for _, b := range c.Blocks {
		if b.ID == id {
			return b, true
		}
	}
	return ChangeBlock{}, false
}
func BlockDigest(b ChangeBlock) string {
	raw := fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s", strings.TrimSpace(b.Chapter), strings.TrimSpace(b.TaskNumber), strings.TrimSpace(b.SourceLocator), strings.TrimSpace(b.ReplacementText), strings.TrimSpace(b.WarningText), strings.TrimSpace(b.AffectedProcedure), strings.TrimSpace(b.EngineeringReference), strings.TrimSpace(b.ApprovalReference), strings.TrimSpace(b.ConfigurationScope))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
