package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (c *RevisionCase) ValidateRules(now time.Time) []RuleResult {
	stamp := now.UTC().Format(time.RFC3339)
	blocks := c.CurrentBlocks()
	results := []RuleResult{
		globalRule("CHANGE_PRESENT", len(blocks) > 0, "已编制变更块", "至少需要一个变更块", "编制至少一个关键内容完整的变更块", stamp),
		globalRule("EFFECTIVITY", c.EffectiveUntil.After(now), "有效期有效", "任务有效期已过", "延长任务有效期后重新校核", stamp),
	}
	for _, b := range blocks {
		results = append(results,
			blockRule("ENGINEERING_REFERENCE", strings.TrimSpace(b.EngineeringReference) != "", "工程依据完整", "缺少工程依据", "补充工程依据", b, stamp),
			blockRule("APPROVAL_REFERENCE", strings.TrimSpace(b.ApprovalReference) != "", "批准引用完整", "缺少批准引用", "补充批准引用", b, stamp),
			blockRule("SCOPE", strings.TrimSpace(b.ConfigurationScope) != "" && scopeCompatible(c.ConfigurationScope, b.ConfigurationScope), "变更构型与任务范围一致", "变更构型超出任务适用范围", "调整变更构型使其处于任务适用范围内", b, stamp),
			blockRule("CROSS_REFERENCE", validCrossReference(b.ReplacementText, b.Chapter, b.TaskNumber), "交叉引用格式有效", "替换内容中的交叉引用不完整", "补全替换内容中的交叉引用", b, stamp),
		)
	}
	sortRuleResults(results)
	c.Checks = results
	c.CheckContentDigest = CurrentContentDigest(c)
	c.CheckedRevision = c.CurrentRevision
	c.ChecksStale = false
	return results
}

func globalRule(code string, passed bool, yes, no, target, stamp string) RuleResult {
	return RuleResult{Code: code, Passed: passed, Message: choose(passed, yes, no), Target: target, CheckedAt: stamp}
}

func blockRule(code string, passed bool, yes, no, target string, b ChangeBlock, stamp string) RuleResult {
	return RuleResult{Code: code, Passed: passed, Message: choose(passed, yes, no), Target: target, BlockID: b.ID, BlockSequence: b.Sequence, Chapter: b.Chapter, TaskNumber: b.TaskNumber, SourceLocator: b.SourceLocator, CheckedAt: stamp}
}

func sortRuleResults(results []RuleResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].BlockSequence != results[j].BlockSequence {
			return results[i].BlockSequence < results[j].BlockSequence
		}
		if results[i].Code != results[j].Code {
			return results[i].Code < results[j].Code
		}
		return results[i].BlockID < results[j].BlockID
	})
}

func CurrentContentDigest(c *RevisionCase) string {
	fact := struct {
		Revision int           `json:"revision"`
		Blocks   []ChangeBlock `json:"blocks"`
	}{Revision: c.CurrentRevision, Blocks: c.CurrentBlocks()}
	raw, _ := json.Marshal(fact)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (c *RevisionCase) SubmissionReadiness(now time.Time) SubmissionReadiness {
	digest := CurrentContentDigest(c)
	result := SubmissionReadiness{ContentDigest: digest, CheckedContentDigest: c.CheckContentDigest, Blockers: []ReadinessBlocker{}}
	result.Stale = c.ChecksStale || len(c.Checks) == 0 || c.CheckedRevision != c.CurrentRevision || c.CheckContentDigest == "" || c.CheckContentDigest != digest
	if c.Status != StatusDraft {
		result.Blockers = append(result.Blockers, ReadinessBlocker{Code: "CURRENT_STATE", Message: "当前流程状态不能提交技术审查", Target: "返回草拟状态后再提交技术审查"})
	}
	if result.Stale {
		result.Blockers = append(result.Blockers, ReadinessBlocker{Code: "CHECKS_STALE", Message: "当前轮内容尚无有效校核结果", Target: "对当前内容重新执行全部规则校核"})
	} else {
		for _, check := range c.Checks {
			if check.Code == "EFFECTIVITY" && !c.EffectiveUntil.After(now) {
				result.FailedCount++
				check.Passed = false
				check.Message = "任务有效期已过"
				check.Target = "延长任务有效期后重新校核"
				result.Blockers = append(result.Blockers, blockerFromRule(check))
				continue
			}
			if check.Passed {
				result.PassedCount++
				continue
			}
			result.FailedCount++
			result.Blockers = append(result.Blockers, blockerFromRule(check))
		}
	}
	if !c.EffectiveUntil.After(now) && !containsBlocker(result.Blockers, "EFFECTIVITY") {
		result.FailedCount++
		result.Blockers = append(result.Blockers, ReadinessBlocker{Code: "EFFECTIVITY", Message: "任务有效期已过", Target: "延长任务有效期后重新校核"})
	}
	sort.SliceStable(result.Blockers, func(i, j int) bool {
		if result.Blockers[i].BlockSequence != result.Blockers[j].BlockSequence {
			return result.Blockers[i].BlockSequence < result.Blockers[j].BlockSequence
		}
		if result.Blockers[i].Code != result.Blockers[j].Code {
			return result.Blockers[i].Code < result.Blockers[j].Code
		}
		return result.Blockers[i].BlockID < result.Blockers[j].BlockID
	})
	result.Ready = len(result.Blockers) == 0
	return result
}

func blockerFromRule(r RuleResult) ReadinessBlocker {
	return ReadinessBlocker{Code: r.Code, Message: r.Message, Target: r.Target, BlockID: r.BlockID, BlockSequence: r.BlockSequence, Chapter: r.Chapter, TaskNumber: r.TaskNumber, SourceLocator: r.SourceLocator}
}

func containsBlocker(items []ReadinessBlocker, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func choose(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func scopeCompatible(caseScope, blockScope string) bool {
	cs := strings.ToUpper(strings.TrimSpace(caseScope))
	bs := strings.ToUpper(strings.TrimSpace(blockScope))
	return bs == cs || strings.Contains(cs, bs) || bs == "ALL"
}

func validCrossReference(text, chapter, task string) bool {
	upper := strings.ToUpper(text)
	if strings.Contains(upper, "REF:") {
		parts := strings.Split(upper, "REF:")
		return len(parts) > 1 && len(strings.TrimSpace(parts[len(parts)-1])) >= 3
	}
	return strings.TrimSpace(chapter) != "" && strings.TrimSpace(task) != ""
}

func AllChecksPass(results []RuleResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func (c *RevisionCase) SubmitReview(now time.Time) error {
	if c.Status != StatusDraft {
		return ErrInvalidState
	}
	readiness := c.SubmissionReadiness(now)
	if !readiness.Ready {
		return SubmissionBlocked{Readiness: readiness}
	}
	if err := c.Transition(StatusReview); err != nil {
		return err
	}
	for i := range c.Rounds {
		if c.Rounds[i].Index == c.CurrentRevision {
			t := now.UTC()
			c.Rounds[i].SubmittedAt = &t
		}
	}
	return nil
}

func CheckSummary(results []RuleResult) string {
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	return fmt.Sprintf("%d/%d 项规则通过", passed, len(results))
}
