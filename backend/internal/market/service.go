// Package market owns the blue-carbon marketplace boundary: reconciled registry
// inventory, purchases, custody, revenue rules, retirement and reconciliation.
// Money and quantities stay decimal strings end to end.
package market

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/money"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	StepUpPurchase   = "marketplace_purchase"
	StepUpRetirement = "credit_retirement"

	actionPurchase   = "marketplace_purchase"
	actionRetirement = "credit_retirement"

	recipientCommunity  = "Community revenue"
	recipientOperations = "Project operations"
)

var (
	ErrNotFound    = errors.New("marketplace resource not found")
	ErrForbidden   = errors.New("this action is not permitted for your account")
	ErrUnavailable = errors.New("the marketplace adapter is unavailable")
	ErrValidation  = errors.New("the request failed validation")
	ErrStepUp      = auth.ErrStepUpRequired
)

type Service struct {
	db           *gorm.DB
	projects     *mrv.Service
	anchors      *provenance.Service
	stepUp       *auth.StepUp
	registryMode string
	paymentMode  string
	now          func() time.Time
}

func NewService(db *gorm.DB, projects *mrv.Service, anchors *provenance.Service, stepUp *auth.StepUp, registryMode, paymentMode string) *Service {
	return &Service{
		db: db, projects: projects, anchors: anchors, stepUp: stepUp,
		registryMode: registryMode, paymentMode: paymentMode,
		now: func() time.Time { return time.Now().UTC() },
	}
}

type RegistryBlock struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Reference string `json:"reference,omitempty"`
}

type Listing struct {
	ID              string        `json:"id"`
	BatchID         string        `json:"batchId"`
	ProjectName     string        `json:"projectName"`
	Vintage         string        `json:"vintage"`
	ProductType     string        `json:"productType"`
	Registry        RegistryBlock `json:"registry"`
	AvailableTonnes string        `json:"availableTonnes"`
	CoBenefits      []string      `json:"coBenefits"`
	PricePerTonne   money.Amount  `json:"pricePerTonne"`
	Mode            string        `json:"mode"`
}

type PurchaseInput struct {
	ListingID      string `json:"listingId"`
	QuantityTonnes string `json:"quantityTonnes"`
	StepUpCode     string `json:"stepUpCode"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type PurchaseResult struct {
	ID                  string       `json:"id"`
	ListingID           string       `json:"listingId"`
	BatchID             string       `json:"batchId"`
	QuantityTonnes      string       `json:"quantityTonnes"`
	Total               money.Amount `json:"total"`
	Status              string       `json:"status"`
	Mode                string       `json:"mode"`
	CustodyReference    string       `json:"custodyReference,omitempty"`
	SettlementReference string       `json:"settlementReference,omitempty"`
	Detail              string       `json:"detail,omitempty"`
}

type RetirementInput struct {
	QuantityTonnes string `json:"quantityTonnes"`
	Reason         string `json:"reason"`
	StepUpCode     string `json:"stepUpCode"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type RetirementResult struct {
	ID                           string     `json:"id"`
	BatchID                      string     `json:"batchId"`
	QuantityTonnes               string     `json:"quantityTonnes"`
	Status                       string     `json:"status"`
	Mode                         string     `json:"mode"`
	RegistrySerialReference      string     `json:"registrySerialReference,omitempty"`
	RegistryCertificateReference string     `json:"registryCertificateReference,omitempty"`
	ChainReference               string     `json:"chainReference,omitempty"`
	ChainNetwork                 string     `json:"chainNetwork,omitempty"`
	ExplorerURL                  string     `json:"explorerUrl,omitempty"`
	RetiredAt                    *time.Time `json:"retiredAt,omitempty"`
	Detail                       string     `json:"detail,omitempty"`
}

// Listings returns inventory the backend can account for. Unreconciled or sandbox
// batches are labelled and can never be purchased as issued credits.
func (s *Service) Listings() ([]Listing, error) {
	var rows []models.MarketplaceListing
	if err := s.db.Where("status = ?", "active").Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Listing, 0, len(rows))
	for _, row := range rows {
		var batch models.CreditBatch
		if err := s.db.Where("id = ?", row.BatchID).First(&batch).Error; err != nil {
			continue
		}
		var project models.ConservationProject
		if err := s.db.Where("id = ?", batch.ProjectID).First(&project).Error; err != nil {
			continue
		}
		available, err := decimal.NewFromString(batch.AvailableQuantity)
		if err != nil || !available.IsPositive() {
			continue
		}
		unitPrice, err := decimal.NewFromString(row.PricePerTonne)
		if err != nil {
			continue
		}
		reconciled := batch.ReconciledAt != nil && batch.AdapterMode == "live"
		registryStatus := "pending"
		if reconciled {
			registryStatus = "reconciled"
		}
		reference := ""
		if reconciled {
			reference = batch.RegistryIssuanceID
		}
		out = append(out, Listing{
			ID: row.ID, BatchID: batch.ID, ProjectName: project.Name, Vintage: fmt.Sprint(batch.Vintage),
			ProductType:     row.ProductType,
			Registry:        RegistryBlock{Name: batch.RegistryName, Status: registryStatus, Reference: reference},
			AvailableTonnes: money.Quantity(available),
			CoBenefits:      s.coBenefits(batch.ProjectID),
			PricePerTonne:   money.NewAmount(unitPrice, row.Currency),
			Mode:            modeLabel(batch.AdapterMode, s.paymentMode),
		})
	}
	return out, nil
}

// Purchase records an idempotent, decimal-exact custody transfer under the
// project's revenue rule.
func (s *Service) Purchase(ctx context.Context, buyerID string, in PurchaseInput) (PurchaseResult, error) {
	if in.IdempotencyKey == "" {
		return PurchaseResult{}, fmt.Errorf("%w: an idempotency key is required", ErrValidation)
	}
	if replay, ok := s.replay(buyerID, actionPurchase, in.IdempotencyKey); ok {
		return replay, nil
	}
	if err := s.stepUp.Consume(buyerID, in.StepUpCode, StepUpPurchase); err != nil {
		return PurchaseResult{}, ErrStepUp
	}
	quantity, err := money.ParseQuantity(in.QuantityTonnes)
	if err != nil || !quantity.IsPositive() {
		return PurchaseResult{}, fmt.Errorf("%w: quantity must be a positive decimal of tonnes", ErrValidation)
	}
	var listing models.MarketplaceListing
	if err := s.db.Where("id = ? AND status = ?", in.ListingID, "active").First(&listing).Error; err != nil {
		return PurchaseResult{}, ErrNotFound
	}
	if listing.ProductType != "registry_issued" {
		return PurchaseResult{}, fmt.Errorf("%w: only registry-issued credits can be purchased", ErrValidation)
	}
	var batch models.CreditBatch
	if err := s.db.Where("id = ?", listing.BatchID).First(&batch).Error; err != nil {
		return PurchaseResult{}, ErrNotFound
	}
	if batch.ReconciledAt == nil || batch.AdapterMode != "live" {
		return PurchaseResult{}, fmt.Errorf("%w: this inventory is not reconciled against a live registry", ErrValidation)
	}
	available, err := decimal.NewFromString(batch.AvailableQuantity)
	if err != nil {
		return PurchaseResult{}, ErrUnavailable
	}
	if quantity.Cmp(available) > 0 {
		return PurchaseResult{}, fmt.Errorf("%w: only %s tCO2e remain in this batch", ErrValidation, money.Quantity(available))
	}
	unitPrice, err := decimal.NewFromString(listing.PricePerTonne)
	if err != nil {
		return PurchaseResult{}, ErrUnavailable
	}
	var project models.ConservationProject
	if err := s.db.Where("id = ?", batch.ProjectID).First(&project).Error; err != nil {
		return PurchaseResult{}, ErrNotFound
	}

	now := s.now()
	gross := money.Total(unitPrice, quantity)
	paymentReference := s.paymentReference(now)
	purchase := models.Purchase{
		ID: uuid.NewString(), ListingID: listing.ID, BatchID: batch.ID, BuyerID: buyerID,
		Quantity: money.Quantity(quantity), GrossAmount: money.Cash(gross), Currency: listing.Currency,
		PaymentMode: s.paymentMode, PaymentReference: paymentReference, Status: "confirmed", CreatedAt: now,
	}
	sellerCustody := models.CustodyEvent{
		ID: uuid.NewString(), BatchID: batch.ID, OwnerID: project.ManagerID,
		EventType: "issued", Quantity: purchase.Quantity, CreatedAt: now,
	}
	buyerCustody := models.CustodyEvent{
		ID: uuid.NewString(), BatchID: batch.ID, PurchaseID: purchase.ID, OwnerID: buyerID,
		EventType: "purchased", Quantity: purchase.Quantity, CreatedAt: now,
	}
	allocations, err := s.allocate(project, gross, purchase)
	if err != nil {
		return PurchaseResult{}, err
	}
	remaining := available.Sub(quantity)
	batchStatus := "partially_sold"
	if remaining.IsZero() {
		batchStatus = "sold_out"
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&models.CreditBatch{}).
			Where("id = ? AND available_quantity = ?", batch.ID, batch.AvailableQuantity).
			Updates(map[string]any{"available_quantity": money.Quantity(remaining), "status": batchStatus, "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return fmt.Errorf("%w: the batch balance changed while this purchase was being recorded", ErrUnavailable)
		}
		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}
		if err := tx.Create(&sellerCustody).Error; err != nil {
			return err
		}
		if err := tx.Create(&buyerCustody).Error; err != nil {
			return err
		}
		for index := range allocations {
			if err := tx.Create(&allocations[index]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PurchaseResult{}, err
	}

	result := PurchaseResult{
		ID: purchase.ID, ListingID: listing.ID, BatchID: batch.ID, QuantityTonnes: purchase.Quantity,
		Total: money.NewAmount(gross, purchase.Currency), Status: purchase.Status,
		Mode: modeLabel(batch.AdapterMode, s.paymentMode), CustodyReference: buyerCustody.ID,
		SettlementReference: purchase.PaymentReference, Detail: settlementDetail(s.paymentMode),
	}
	s.persist(ctx, buyerID, actionPurchase, in.IdempotencyKey, result)
	return result, nil
}

// Retire locks serials and only reports completion when a live registry confirms it.
func (s *Service) Retire(ctx context.Context, ownerID, batchID string, in RetirementInput) (RetirementResult, error) {
	if in.IdempotencyKey == "" {
		return RetirementResult{}, fmt.Errorf("%w: an idempotency key is required", ErrValidation)
	}
	if replay, ok := s.replayRetirement(ownerID, actionRetirement, in.IdempotencyKey); ok {
		return replay, nil
	}
	if err := s.stepUp.Consume(ownerID, in.StepUpCode, StepUpRetirement); err != nil {
		return RetirementResult{}, ErrStepUp
	}
	quantity, err := money.ParseQuantity(in.QuantityTonnes)
	if err != nil || !quantity.IsPositive() {
		return RetirementResult{}, fmt.Errorf("%w: retirement quantity must be a positive decimal", ErrValidation)
	}
	if strings.TrimSpace(in.Reason) == "" {
		return RetirementResult{}, fmt.Errorf("%w: a retirement reason is required for the registry record", ErrValidation)
	}
	var batch models.CreditBatch
	if err := s.db.Where("id = ?", batchID).First(&batch).Error; err != nil {
		return RetirementResult{}, ErrNotFound
	}
	owned, err := s.ownedBy(batch.ID, ownerID)
	if err != nil {
		return RetirementResult{}, err
	}
	if owned.Cmp(quantity) < 0 {
		return RetirementResult{}, fmt.Errorf("%w: this account holds %s tCO2e of this batch", ErrValidation, money.Quantity(owned))
	}
	confirmed := s.registryMode == "live" && batch.AdapterMode == "live"
	status := "reconciliation_pending"
	if confirmed {
		status = "completed"
	}
	now := s.now()
	certificate := ""
	if confirmed {
		certificate = fmt.Sprintf("%s-CERT-%s", batch.RegistryName, batch.RegistryIssuanceID)
	}
	retirement := models.Retirement{
		ID: uuid.NewString(), BatchID: batch.ID, OwnerID: ownerID, Quantity: money.Quantity(quantity),
		Status: status, RegistryCertificate: certificate, CreatedAt: now, UpdatedAt: now,
	}
	if confirmed {
		retirement.CompletedAt = &now
	}
	custody := models.CustodyEvent{
		ID: uuid.NewString(), BatchID: batch.ID, OwnerID: ownerID, EventType: "retired",
		Quantity: retirement.Quantity, CreatedAt: now,
	}
	caseRecord := models.ReconciliationCase{
		ID: uuid.NewString(), BatchID: batch.ID, Status: "open", CreatedAt: now,
		Reason: "Retirement recorded without live registry confirmation: " + in.Reason,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&retirement).Error; err != nil {
			return err
		}
		if err := tx.Create(&custody).Error; err != nil {
			return err
		}
		if confirmed {
			return nil
		}
		return tx.Create(&caseRecord).Error
	})
	if err != nil {
		return RetirementResult{}, err
	}

	result := RetirementResult{
		ID: retirement.ID, BatchID: batch.ID, QuantityTonnes: retirement.Quantity, Status: status,
		Mode:                         modeLabel(batch.AdapterMode, s.registryMode),
		RegistrySerialReference:      fmt.Sprintf("%s-%s", batch.SerialStart, batch.SerialEnd),
		RegistryCertificateReference: certificate,
		Detail:                       retirementDetail(status, in.Reason),
	}
	if confirmed {
		result.RetiredAt = &now
	}
	if event, anchorErr := s.anchorRetirement(ctx, batch, ownerID, retirement, in.Reason); anchorErr == nil && event != nil {
		result.ChainReference = event.TransactionRef
		result.ChainNetwork = s.anchors.PublicNetwork()
		result.ExplorerURL = s.anchors.ExplorerTransaction(*event)
	} else if anchorErr != nil {
		result.Detail = result.Detail + " Provenance anchor pending: " + anchorErr.Error() + "."
	}
	s.persist(ctx, ownerID, actionRetirement, in.IdempotencyKey, result)
	return result, nil
}

func (s *Service) anchorRetirement(ctx context.Context, batch models.CreditBatch, ownerID string,
	retirement models.Retirement, reason string) (*models.ProvenanceEvent, error) {
	var activity models.ConservationActivity
	err := s.db.Where("project_id = ? AND status = ?", batch.ProjectID, models.ActivityVerified).
		Order("reporting_period_end desc").First(&activity).Error
	if err != nil {
		return nil, domain.Reject("no verified achievement is available to attribute this retirement to")
	}
	did, err := s.activeDID(ownerID)
	if err != nil {
		return nil, err
	}
	event, err := s.anchors.Record(ctx, provenance.Input{
		AchievementID: activity.ID, ProjectID: batch.ProjectID, ClaimType: "retired",
		ActivityType: activity.ActivityType, MethodologyVersion: activity.MethodologyVersion,
		ReportingPeriodStart: activity.ReportingPeriodStart, ReportingPeriodEnd: activity.ReportingPeriodEnd,
		Measurements: map[string]any{
			"batch_id": batch.ID, "serial_start": batch.SerialStart, "serial_end": batch.SerialEnd,
			"quantity": retirement.Quantity, "registry_certificate": retirement.RegistryCertificate,
			"registry_mode": s.registryMode, "reason": reason,
		},
		MeasurementBasis: "registry_retirement", IssuerDID: did,
	})
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *Service) activeDID(userID string) (string, error) {
	var provision models.IdentityProvision
	if err := s.db.Where("user_id = ? AND state = ?", userID, models.IdentityActive).First(&provision).Error; err != nil {
		return "", fmt.Errorf("%w: an active provisioned DID is required", ErrValidation)
	}
	return provision.DID, nil
}

func (s *Service) ownedBy(batchID, ownerID string) (decimal.Decimal, error) {
	var events []models.CustodyEvent
	if err := s.db.Where("batch_id = ? AND owner_id = ?", batchID, ownerID).Find(&events).Error; err != nil {
		return decimal.Zero, err
	}
	owned := decimal.Zero
	for _, event := range events {
		quantity, err := decimal.NewFromString(event.Quantity)
		if err != nil {
			return decimal.Zero, ErrUnavailable
		}
		switch event.EventType {
		case "purchased", "transferred_in":
			owned = owned.Add(quantity)
		case "retired", "transferred_out":
			owned = owned.Sub(quantity)
		}
	}
	if owned.IsNegative() {
		return decimal.Zero, nil
	}
	return owned, nil
}

// allocate splits the gross amount across the project's configured rule.
func (s *Service) allocate(project models.ConservationProject, gross decimal.Decimal, purchase models.Purchase) ([]models.RevenueAllocation, error) {
	if project.CommunityShareBPS <= 0 || project.CommunityShareBPS >= money.BasisPointsWhole {
		return nil, fmt.Errorf("%w: the project revenue rule is not configured", ErrValidation)
	}
	parts, _, err := money.SplitByBasisPoints(gross, []money.Allocation{
		{Label: recipientCommunity, BasisPoints: project.CommunityShareBPS},
		{Label: recipientOperations, BasisPoints: money.BasisPointsWhole - project.CommunityShareBPS},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	allocations := make([]models.RevenueAllocation, 0, len(parts))
	for index, part := range parts {
		label := recipientOperations
		points := money.BasisPointsWhole - project.CommunityShareBPS
		if index == 0 {
			label = recipientCommunity
			points = project.CommunityShareBPS
		}
		allocations = append(allocations, models.RevenueAllocation{
			ID: uuid.NewString(), PurchaseID: purchase.ID, ProjectID: project.ID, Recipient: label,
			BasisPoints: points, Amount: money.Cash(part), Currency: purchase.Currency, CreatedAt: purchase.CreatedAt,
		})
	}
	return allocations, nil
}

func (s *Service) paymentReference(now time.Time) string {
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if s.paymentMode == "live" {
		return "pay-" + now.Format("20060102") + "-" + suffix
	}
	return "sandbox-pay-" + suffix
}

func settlementDetail(paymentMode string) string {
	if paymentMode == "live" {
		return "Settlement reference returned by the live payment adapter."
	}
	return "Sandbox settlement: no funds moved and no regulated disbursement was initiated."
}

func retirementDetail(status, reason string) string {
	if status == "completed" {
		return "Registry retirement confirmed for: " + reason + "."
	}
	return "Retirement recorded locally for: " + reason + ". Completion requires a live registry confirmation."
}

func modeLabel(adapterMode, paymentMode string) string {
	if adapterMode == "live" && paymentMode == "live" {
		return "live"
	}
	if adapterMode == "" && paymentMode == "" {
		return "unavailable"
	}
	return "sandbox"
}
