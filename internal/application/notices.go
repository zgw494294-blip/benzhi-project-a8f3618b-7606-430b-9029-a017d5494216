package application

import (
	"context"
	"revisiongate/internal/domain"
	"sort"
	"strings"
)

type NoticeQuery struct {
	SerialNumber     string
	ManualNumber     string
	AircraftModel    string
	Configuration    string
	VerificationCode string
	Page             int
	PageSize         int
}

type NoticeSearchItem struct {
	CaseID             string                    `json:"caseId"`
	RevisionNumber     string                    `json:"revisionNumber"`
	ManualNumber       string                    `json:"manualNumber"`
	AircraftModels     []string                  `json:"aircraftModels"`
	ConfigurationScope string                    `json:"configurationScope"`
	Notice             *domain.EffectivityNotice `json:"notice"`
}

type NoticeSearchResult struct {
	Items    []NoticeSearchItem `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int                `json:"total"`
}

type NoticeVerificationView struct {
	Item         NoticeSearchItem          `json:"item"`
	Verification domain.NoticeVerification `json:"verification"`
	FieldUsable  bool                      `json:"fieldUsable"`
}

func (s *Service) SearchNotices(ctx context.Context, query NoticeQuery) (*NoticeSearchResult, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 {
		return nil, Translate(domain.Violation{Field: "page", Message: "page 必须为正整数"})
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		return nil, Translate(domain.Violation{Field: "pageSize", Message: "pageSize 必须在 1 到 100 之间"})
	}
	cases, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]NoticeSearchItem, 0)
	for _, c := range cases {
		if c.Status != domain.StatusEffective || c.Notice == nil {
			continue
		}
		if !containsFold(c.Notice.SerialNumber, query.SerialNumber) || !containsFold(c.ManualNumber, query.ManualNumber) || !containsFold(c.ConfigurationScope, query.Configuration) {
			continue
		}
		if strings.TrimSpace(query.AircraftModel) != "" && !listContainsFold(c.AircraftModels, query.AircraftModel) {
			continue
		}
		if strings.TrimSpace(query.VerificationCode) != "" && domain.NormalizeVerificationCode(query.VerificationCode) != domain.NormalizeVerificationCode(c.Notice.VerificationCode) {
			continue
		}
		items = append(items, noticeItem(c))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Notice.IssuedAt.Equal(items[j].Notice.IssuedAt) {
			return items[i].Notice.IssuedAt.After(items[j].Notice.IssuedAt)
		}
		return items[i].Notice.SerialNumber < items[j].Notice.SerialNumber
	})
	result := &NoticeSearchResult{Items: []NoticeSearchItem{}, Page: query.Page, PageSize: query.PageSize, Total: len(items)}
	start := (query.Page - 1) * query.PageSize
	if start >= len(items) {
		return result, nil
	}
	end := start + query.PageSize
	if end > len(items) {
		end = len(items)
	}
	result.Items = items[start:end]
	return result, nil
}

func (s *Service) VerifyEffectivityNotice(ctx context.Context, noticeID, code string) (*NoticeVerificationView, error) {
	s.noticeMu.RLock()
	caseID, ok := s.noticeLookup[noticeID]
	s.noticeMu.RUnlock()
	if ok {
		c, err := s.Get(ctx, caseID)
		if err != nil {
			return nil, err
		}
		verification := domain.VerifyNoticeAt(c, code, s.now())
		return &NoticeVerificationView{Item: noticeItem(c), Verification: verification, FieldUsable: verification.Matched && verification.Status == domain.NoticeCurrent}, nil
	}
	cases, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	s.noticeMu.Lock()
	defer s.noticeMu.Unlock()
	var matched *domain.RevisionCase
	for _, c := range cases {
		if c.Status != domain.StatusEffective || c.Notice == nil {
			continue
		}
		s.noticeLookup[c.Notice.ID] = c.ID
		s.noticeLookup[c.Notice.SerialNumber] = c.ID
		if matched == nil && (c.Notice.ID == noticeID || c.Notice.SerialNumber == noticeID) {
			matched = c
		}
	}
	if matched == nil {
		return nil, Translate(ErrNotFound)
	}
	verification := domain.VerifyNoticeAt(matched, code, s.now())
	return &NoticeVerificationView{Item: noticeItem(matched), Verification: verification, FieldUsable: verification.Matched && verification.Status == domain.NoticeCurrent}, nil
}

func noticeItem(c *domain.RevisionCase) NoticeSearchItem {
	return NoticeSearchItem{CaseID: c.ID, RevisionNumber: c.RevisionNumber, ManualNumber: c.ManualNumber, AircraftModels: append([]string(nil), c.AircraftModels...), ConfigurationScope: c.ConfigurationScope, Notice: c.Notice}
}
func containsFold(value, filter string) bool {
	return strings.TrimSpace(filter) == "" || strings.Contains(strings.ToUpper(value), strings.ToUpper(strings.TrimSpace(filter)))
}
func listContainsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}
