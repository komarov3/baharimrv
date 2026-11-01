package market

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/baharilink/baharimrv/internal/canon"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/money"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Idempotent results are stored as sync actions so a retried request returns the
// original outcome instead of duplicating a custody transfer.
func (s *Service) replay(userID, action, key string) (PurchaseResult, bool) {
	var record models.SyncAction
	err := s.db.Where("user_id = ? AND client_id = ? AND action_type = ? AND status = ?",
		userID, key, action, "completed").First(&record).Error
	if err != nil {
		return PurchaseResult{}, false
	}
	var result PurchaseResult
	if json.Unmarshal(record.Response, &result) != nil || result.ID == "" {
		return PurchaseResult{}, false
	}
	return result, true
}

func (s *Service) replayRetirement(userID, action, key string) (RetirementResult, bool) {
	var record models.SyncAction
	err := s.db.Where("user_id = ? AND client_id = ? AND action_type = ? AND status = ?",
		userID, key, action, "completed").First(&record).Error
	if err != nil {
		return RetirementResult{}, false
	}
	var result RetirementResult
	if json.Unmarshal(record.Response, &result) != nil || result.ID == "" {
		return RetirementResult{}, false
	}
	return result, true
}

func (s *Service) persist(ctx context.Context, userID, action, key string, result any) {
	body, err := canon.JSON(result)
	if err != nil {
		return
	}
	now := s.now()
	record := models.SyncAction{
		ID: uuid.NewString(), UserID: userID, ClientID: key, ActionType: action,
		RequestHash: canon.HashBytes(body), Status: "completed", Response: datatypes.JSON(body),
		CreatedAt: now, UpdatedAt: now,
	}
	if createErr := s.db.WithContext(ctx).Create(&record).Error; createErr != nil {
		return
	}
}

type WalletBalances struct {
	TourismContributions money.Amount `json:"tourismContributions"`
	CarbonCreditSales    money.Amount `json:"carbonCreditSales"`
	CommunityShare       money.Amount `json:"communityShare"`
}

type RevenueAllocationView struct {
	Label   string       `json:"label"`
	Percent string       `json:"percent"`
	Amount  money.Amount `json:"amount"`
}

type RevenueRule struct {
	ID               string                  `json:"id"`
	Label            string                  `json:"label"`
	CommunityPercent string                  `json:"communityPercent"`
	EffectiveFrom    time.Time               `json:"effectiveFrom"`
	Allocations      []RevenueAllocationView `json:"allocations"`
}

type LedgerEntry struct {
	ID          string       `json:"id"`
	Kind        string       `json:"kind"`
	Description string       `json:"description"`
	Amount      money.Amount `json:"amount"`
	Status      string       `json:"status"`
	Source      string       `json:"source"`
	OccurredAt  time.Time    `json:"occurredAt"`
	Reference   string       `json:"reference,omitempty"`
}

type Wallet struct {
	ProjectID      string         `json:"projectId"`
	Mode           string         `json:"mode"`
	Balances       WalletBalances `json:"balances"`
	RevenueRule    *RevenueRule   `json:"revenueRule,omitempty"`
	Entries        []LedgerEntry  `json:"entries"`
	LastImportedAt *time.Time     `json:"lastImportedAt,omitempty"`
}

type ReconciliationIssue struct {
	ID       string `json:"id"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
}

type Reconciliation struct {
	ProjectID string                `json:"projectId"`
	Status    string                `json:"status"`
	CheckedAt *time.Time            `json:"checkedAt,omitempty"`
	Issues    []ReconciliationIssue `json:"issues"`
}

// Wallet is the read-only financial projection for a project. Nothing here is
// synthesised: every entry comes from an imported or recorded row.
func (s *Service) Wallet(projectID, userID string) (Wallet, error) {
	if err := s.projects.AssertProjectAccess(userID, projectID); err != nil {
		return Wallet{}, ErrForbidden
	}
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		return Wallet{}, ErrNotFound
	}
	var imports []models.FinancialImport
	if err := s.db.Where("project_id = ?", projectID).Order("occurred_at desc").Find(&imports).Error; err != nil {
		return Wallet{}, err
	}
	purchases, allocations, err := s.projectSales(projectID)
	if err != nil {
		return Wallet{}, err
	}

	tourism := decimal.Zero
	for _, record := range imports {
		if amount, parseErr := decimal.NewFromString(record.Amount); parseErr == nil {
			tourism = tourism.Add(amount)
		}
	}
	sales := decimal.Zero
	for _, purchase := range purchases {
		if amount, parseErr := decimal.NewFromString(purchase.GrossAmount); parseErr == nil {
			sales = sales.Add(amount)
		}
	}
	community := decimal.Zero
	for _, allocation := range allocations {
		if allocation.Recipient != recipientCommunity {
			continue
		}
		if amount, parseErr := decimal.NewFromString(allocation.Amount); parseErr == nil {
			community = community.Add(amount)
		}
	}

	currency := projectCurrency(imports, purchases, allocations)
	entries := buildEntries(imports, purchases, allocations, currency)
	wallet := Wallet{
		ProjectID: projectID, Mode: modeLabel(s.registryMode, s.paymentMode),
		Balances: WalletBalances{
			TourismContributions: money.NewAmount(tourism, currency),
			CarbonCreditSales:    money.NewAmount(sales, currency),
			CommunityShare:       money.NewAmount(community, currency),
		},
		Entries: entries,
	}
	if len(entries) > 0 {
		latest := entries[0].OccurredAt
		wallet.LastImportedAt = &latest
	}
	if project.CommunityShareBPS > 0 {
		communityShare := money.Share(sales, project.CommunityShareBPS)
		operations := sales.Sub(communityShare)
		wallet.RevenueRule = &RevenueRule{
			ID: projectID + "/revenue-rule", Label: "Project revenue rule (configured basis points)",
			CommunityPercent: money.Percent(project.CommunityShareBPS), EffectiveFrom: project.CreatedAt,
			Allocations: []RevenueAllocationView{
				{Label: recipientCommunity, Percent: money.Percent(project.CommunityShareBPS), Amount: money.NewAmount(communityShare, currency)},
				{Label: recipientOperations, Percent: money.Percent(money.BasisPointsWhole - project.CommunityShareBPS), Amount: money.NewAmount(operations, currency)},
			},
		}
	}
	return wallet, nil
}

// Reconciliation compares stored batch balances against recorded purchases and
// any open reconciliation cases.
func (s *Service) Reconciliation(projectID, userID string) (Reconciliation, error) {
	if err := s.projects.AssertProjectAccess(userID, projectID); err != nil {
		return Reconciliation{}, ErrForbidden
	}
	var batches []models.CreditBatch
	if err := s.db.Where("project_id = ?", projectID).Find(&batches).Error; err != nil {
		return Reconciliation{}, err
	}
	status := "reconciled"
	issues := make([]ReconciliationIssue, 0)
	for _, batch := range batches {
		issue, mismatched := reconcileBatch(s.db, batch)
		if mismatched {
			issues = append(issues, issue)
			status = "mismatch"
		}
	}
	var cases []models.ReconciliationCase
	if err := s.db.Joins("JOIN credit_batches ON credit_batches.id = reconciliation_cases.batch_id").
		Where("credit_batches.project_id = ? AND reconciliation_cases.status = ?", projectID, "open").
		Find(&cases).Error; err != nil {
		return Reconciliation{}, err
	}
	for _, record := range cases {
		issues = append(issues, ReconciliationIssue{ID: record.ID, Detail: record.Reason, Severity: "warning"})
	}
	if status == "reconciled" && len(cases) > 0 {
		status = "pending"
	}
	if s.registryMode != "live" {
		status = "unavailable"
		issues = append(issues, ReconciliationIssue{ID: "registry-mode", Severity: "warning",
			Detail: "Registry reconciliation needs a live registry adapter. Sandbox balances are not evidence of issued credits."})
	}
	if len(batches) == 0 {
		status = "unavailable"
		issues = append(issues, ReconciliationIssue{ID: "no-batches", Severity: "warning",
			Detail: "No registry batch has been imported for this project, so there is nothing to reconcile."})
	}
	checked := s.now()
	return Reconciliation{ProjectID: projectID, Status: status, CheckedAt: &checked, Issues: issues}, nil
}

func reconcileBatch(db *gorm.DB, batch models.CreditBatch) (ReconciliationIssue, bool) {
	issued, err := decimal.NewFromString(batch.Quantity)
	if err != nil {
		return ReconciliationIssue{ID: batch.ID + "-quantity", Severity: "critical",
			Detail: "Stored batch quantity is unreadable."}, true
	}
	available, err := decimal.NewFromString(batch.AvailableQuantity)
	if err != nil {
		return ReconciliationIssue{ID: batch.ID + "-available", Severity: "critical",
			Detail: "Stored available quantity is unreadable."}, true
	}
	var purchases []models.Purchase
	if err := db.Where("batch_id = ?", batch.ID).Find(&purchases).Error; err != nil {
		return ReconciliationIssue{}, false
	}
	sold := decimal.Zero
	for _, purchase := range purchases {
		if quantity, parseErr := decimal.NewFromString(purchase.Quantity); parseErr == nil {
			sold = sold.Add(quantity)
		}
	}
	if issued.Sub(sold).Compare(available) != 0 {
		return ReconciliationIssue{ID: batch.ID + "-balance", Severity: "critical", Detail: fmt.Sprintf(
			"Batch %s: issued %s minus purchased %s does not equal available %s.",
			batch.ID, money.Quantity(issued), money.Quantity(sold), money.Quantity(available))}, true
	}
	if available.IsNegative() {
		return ReconciliationIssue{ID: batch.ID + "-negative", Severity: "critical",
			Detail: fmt.Sprintf("Batch %s has a negative available balance.", batch.ID)}, true
	}
	return ReconciliationIssue{}, false
}

func (s *Service) projectSales(projectID string) ([]models.Purchase, []models.RevenueAllocation, error) {
	var purchases []models.Purchase
	err := s.db.Joins("JOIN credit_batches ON credit_batches.id = purchases.batch_id").
		Where("credit_batches.project_id = ?", projectID).Order("purchases.created_at desc").Find(&purchases).Error
	if err != nil {
		return nil, nil, err
	}
	var allocations []models.RevenueAllocation
	if err := s.db.Where("project_id = ?", projectID).Order("created_at desc").Find(&allocations).Error; err != nil {
		return nil, nil, err
	}
	return purchases, allocations, nil
}

func buildEntries(imports []models.FinancialImport, purchases []models.Purchase,
	allocations []models.RevenueAllocation, currency string) []LedgerEntry {
	entries := make([]LedgerEntry, 0, len(imports)+len(purchases)+len(allocations))
	for _, record := range imports {
		amount, err := decimal.NewFromString(record.Amount)
		if err != nil {
			continue
		}
		entries = append(entries, LedgerEntry{
			ID: record.ID, Kind: "tourism_contribution",
			Description: "BahariLink tourism contribution imported from " + record.Source,
			Amount:      money.NewAmount(amount, record.Currency), Status: "imported",
			Source: record.Source, OccurredAt: record.OccurredAt, Reference: record.ExternalID,
		})
	}
	for _, purchase := range purchases {
		amount, err := decimal.NewFromString(purchase.GrossAmount)
		if err != nil {
			continue
		}
		tonnes, _ := decimal.NewFromString(purchase.Quantity)
		status := "pending"
		if purchase.Status == "confirmed" {
			status = "reconciled"
		}
		entries = append(entries, LedgerEntry{
			ID: purchase.ID, Kind: "carbon_credit_sale",
			Description: fmt.Sprintf("Registry-issued credit sale of %s tCO2e", money.Quantity(tonnes)),
			Amount:      money.NewAmount(amount, purchase.Currency), Status: status,
			Source: purchase.PaymentMode, OccurredAt: purchase.CreatedAt, Reference: purchase.PaymentReference,
		})
	}
	for _, allocation := range allocations {
		amount, err := decimal.NewFromString(allocation.Amount)
		if err != nil {
			continue
		}
		entries = append(entries, LedgerEntry{
			ID: allocation.ID, Kind: "community_allocation",
			Description: allocation.Recipient + " allocation at " + money.Percent(allocation.BasisPoints) + "%",
			Amount:      money.NewAmount(amount, allocation.Currency), Status: "reconciled",
			Source: "revenue_rule", OccurredAt: allocation.CreatedAt, Reference: allocation.PurchaseID,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].OccurredAt.After(entries[j].OccurredAt) })
	return entries
}

func projectCurrency(imports []models.FinancialImport, purchases []models.Purchase, allocations []models.RevenueAllocation) string {
	for _, record := range imports {
		if len(record.Currency) == 3 {
			return record.Currency
		}
	}
	for _, purchase := range purchases {
		if len(purchase.Currency) == 3 {
			return purchase.Currency
		}
	}
	for _, allocation := range allocations {
		if len(allocation.Currency) == 3 {
			return allocation.Currency
		}
	}
	return "USD"
}

// coBenefits derives benefit statements from stored data rather than marketing copy.
func (s *Service) coBenefits(projectID string) []string {
	benefits := make([]string, 0, 3)
	var project models.ConservationProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err == nil && project.CommunityShareBPS > 0 {
		benefits = append(benefits, "Community revenue share of "+money.Percent(project.CommunityShareBPS)+"%")
	}
	var verified int64
	if err := s.db.Model(&models.ConservationActivity{}).Where("project_id = ? AND status = ?", projectID, models.ActivityVerified).
		Count(&verified).Error; err == nil && verified > 0 {
		benefits = append(benefits, fmt.Sprintf("%d independently verified achievement(s)", verified))
	}
	if species := s.speciesObserved(projectID); len(species) > 0 {
		benefits = append(benefits, "Species recorded: "+species)
	}
	if len(benefits) == 0 {
		benefits = append(benefits, "No co-benefit data recorded yet")
	}
	return benefits
}

func (s *Service) speciesObserved(projectID string) string {
	var records []models.MonitoringRecord
	err := s.db.Joins("JOIN conservation_activities ON conservation_activities.id = monitoring_records.activity_id").
		Where("conservation_activities.project_id = ?", projectID).Find(&records).Error
	if err != nil {
		return ""
	}
	seen := make(map[string]bool)
	names := make([]string, 0, 3)
	for _, record := range records {
		var species []string
		if json.Unmarshal(record.Species, &species) != nil {
			continue
		}
		for _, name := range species {
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	if len(names) > 3 {
		names = names[:3]
	}
	return joinNames(names)
}

func joinNames(names []string) string {
	out := ""
	for index, name := range names {
		if index > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
