package proof

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/jung-kurt/gofpdf"
	"github.com/skip2/go-qrcode"
)

func renderQR(target string) ([]byte, error) {
	return qrcode.Encode(target, qrcode.Medium, 512)
}

// renderPDF draws the proof from the same snapshot that the JSON manifest carries.
func renderPDF(snapshot Snapshot, networkLabel string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(45, 74, 78)
	pdf.Cell(0, 6, "BAHARI MRV  /  CONSERVATION ACHIEVEMENT PROOF")
	pdf.Ln(9)

	pdf.SetFont("Helvetica", "B", 19)
	pdf.SetTextColor(19, 42, 46)
	heading := "Manager-attested completion claim"
	if snapshot.ProofType == TypeVerified {
		heading = "Independently verified achievement"
	}
	pdf.MultiCell(0, 9, heading, "", "L", false)
	pdf.Ln(3)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(40, 40, 40)
	pdf.MultiCell(0, 6, snapshot.Claim.Summary, "", "L", false)
	pdf.Ln(4)

	pdf.SetDrawColor(198, 210, 205)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(5)

	identifiers := [][2]string{
		{"Proof ID", snapshot.ProofID},
		{"Version", fmt.Sprintf("v%d", snapshot.Version)},
		{"Generated", snapshot.GeneratedAt.UTC().Format(time.RFC3339)},
		{"Project", snapshot.ProjectName},
		{"Achievement ID", snapshot.AchievementID},
		{"Issuer DID", snapshot.IssuerDID},
	}
	for _, pair := range identifiers {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.Cell(45, 6, strings.ToUpper(pair[0]))
		pdf.SetFont("Courier", "", 9)
		pdf.SetTextColor(20, 20, 20)
		pdf.MultiCell(0, 6, pair[1], "", "L", false)
	}
	pdf.Ln(3)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(19, 42, 46)
	pdf.Cell(0, 8, "Claim detail")
	pdf.Ln(9)
	claim := [][2]string{
		{"Activity type", snapshot.Claim.ActivityType},
		{"Methodology version", snapshot.Claim.MethodologyVersion},
		{"Reporting period", snapshot.Claim.ReportingStart.UTC().Format("2006-01-02") + " to " + snapshot.Claim.ReportingEnd.UTC().Format("2006-01-02")},
		{"Evidence root", snapshot.Claim.EvidenceRoot},
		{"Satellite corroboration", hashOrNone(snapshot.Claim.SatelliteHash)},
		{"Unsigned records", fmt.Sprintf("%d", snapshot.Claim.UnsignedRecords)},
		{"Low-accuracy records", fmt.Sprintf("%d", snapshot.Claim.LowAccuracyRecords)},
	}
	writePairs(pdf, claim)

	if snapshot.Verifier != nil {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Independent attestation")
		pdf.Ln(9)
		verifier := [][2]string{
			{"Organization", snapshot.Verifier.Organization},
			{"Decision", snapshot.Verifier.Decision},
			{"Timestamp", snapshot.Verifier.Timestamp.UTC().Format(time.RFC3339)},
			{"Verifier DID", snapshot.Verifier.DID},
			{"Attestation hash", snapshot.Verifier.Hash},
		}
		if snapshot.Verifier.Revoked {
			verifier = append(verifier, [2]string{"Status", "REVOKED"})
		}
		writePairs(pdf, verifier)
	}

	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 8, "Chain commitment")
	pdf.Ln(9)
	if snapshot.Chain != nil && snapshot.Chain.Confirmed {
		writePairs(pdf, [][2]string{
			{"Network", networkLabel},
			{"Transaction", snapshot.Chain.TransactionReference},
			{"Commitment", snapshot.Chain.Commitment},
			{"Explorer", snapshot.Chain.ExplorerURL},
		})
	} else {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(120, 60, 20)
		pdf.MultiCell(0, 6, "No confirmed chain commitment. The claim is recorded but its anchoring is pending or unavailable.", "", "L", false)
	}

	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(19, 42, 46)
	pdf.Cell(0, 8, "Evidence manifest")
	pdf.Ln(9)
	if len(snapshot.EvidenceManifest) == 0 {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.MultiCell(0, 6, "No evidence entries are included in this snapshot.", "", "L", false)
	} else {
		for _, entry := range snapshot.EvidenceManifest {
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(20, 20, 20)
			pdf.MultiCell(0, 5, fmt.Sprintf("%s  (%s)  %s", entry.Name, entry.Access, entry.SHA256), "", "L", false)
		}
	}

	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 8, "Provenance history")
	pdf.Ln(9)
	for _, event := range snapshot.Events {
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, fmt.Sprintf("%s  %s  %s  %s", event.Timestamp.UTC().Format(time.RFC3339), event.Type, event.Status, event.Detail), "", "L", false)
	}

	if len(snapshot.RegistryReferences) > 0 {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Registry references")
		pdf.Ln(9)
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, strings.Join(snapshot.RegistryReferences, ", "), "", "L", false)
	}

	if snapshot.SandboxNotice != "" {
		pdf.Ln(3)
		pdf.SetFillColor(247, 242, 230)
		pdf.SetTextColor(120, 60, 20)
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 6, "INTEGRATION NOTICE: "+snapshot.SandboxNotice, "", "L", true)
	}

	pdf.Ln(3)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(110, 110, 110)
	pdf.MultiCell(0, 4, "Verify this document independently at the URL printed below. A transaction hash alone is not proof of restoration.", "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("the proof PDF could not be rendered")
	}
	return out.Bytes(), nil
}

func writePairs(pdf *gofpdf.Fpdf, pairs [][2]string) {
	for _, pair := range pairs {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.Cell(50, 6, pair[0])
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(20, 20, 20)
		pdf.MultiCell(0, 6, pair[1], "", "L", false)
	}
}

func hashOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// renderZip bundles the PDF, the canonical manifest, instructions, and only the
// evidence entries marked public.
func (s *Service) renderZip(snapshot Snapshot, document models.ProofDocument, base string) ([]byte, error) {
	pdf, err := renderPDF(snapshot, proofNetwork(s.anchors.Network()))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	add := func(name string, body []byte) error {
		file, createErr := writer.Create(name)
		if createErr != nil {
			return createErr
		}
		_, writeErr := file.Write(body)
		return writeErr
	}
	if err := add(base+".pdf", pdf); err != nil {
		return nil, err
	}
	if err := add(base+".json", append(document.Snapshot, '\n')); err != nil {
		return nil, err
	}
	if err := add("README.txt", []byte(readme(snapshot, document))); err != nil {
		return nil, err
	}
	manifest, marshalErr := json.MarshalIndent(snapshot.EvidenceManifest, "", "  ")
	if marshalErr == nil {
		_ = add("evidence/manifest.json", append(manifest, '\n'))
	}
	for _, entry := range snapshot.EvidenceManifest {
		if entry.Access != "public" {
			continue
		}
		var object models.EvidenceObject
		if err := s.db.Where("sha256 = ?", entry.SHA256).First(&object).Error; err != nil {
			continue
		}
		reader, _, openErr := s.store.Open(object.StorageURI)
		if openErr != nil {
			continue
		}
		body, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			continue
		}
		if err := add("evidence/public/"+safeArchiveName(entry.Name), body); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func readme(snapshot Snapshot, document models.ProofDocument) string {
	lines := []string{
		"BahariMRV achievement proof",
		"",
		"Proof ID:        " + document.ID,
		"Version:         v" + fmt.Sprint(document.Version),
		"Proof type:      " + document.ProofType,
		"Status:          " + document.Status,
		"Manifest hash:   " + document.ManifestHash,
		"Generated:       " + snapshot.GeneratedAt.UTC().Format(time.RFC3339),
		"Achievement ID:  " + snapshot.AchievementID,
		"Issuer DID:      " + snapshot.IssuerDID,
		"",
		"Contents:",
		"  proof PDF and the canonical JSON manifest rendered from the same snapshot",
		"  evidence/manifest.json listing every entry, with only public files included",
		"",
		"Restricted evidence is excluded from this package. A transaction hash alone is",
		"not proof of restoration; run the independent checks at the public proof URL.",
	}
	if snapshot.SandboxNotice != "" {
		lines = append(lines, "", "Integration notice: "+snapshot.SandboxNotice)
	}
	return strings.Join(lines, "\n") + "\n"
}

func safeArchiveName(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		case r == ' ':
			return '_'
		default:
			return -1
		}
	}, name)
	cleaned = strings.Trim(cleaned, ".")
	if cleaned == "" {
		return "evidence-file"
	}
	return cleaned
}
