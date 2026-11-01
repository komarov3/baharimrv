# UI Redesign: Wireframe implementation and Material 3 polish

Redesigned Android screens to match wireframe specifications (C1-C4) while enhancing login, profile, and design system components with professional Material 3 treatment, custom BahariMRV logo, and proper attribution.

The implementation introduces new design system components (VerifiedBadge, StageNumberCircle, WalletRevenueStream) and applies wireframe-specified layouts across HomeScreen (2x2 metrics grid, wallet preview, alerts), MonitorScreen (GPS badge positioning, evidence count formatting), and CreditsScreen (verification pipeline stages with checkmarks, marketplace listings with verified badges). LoginScreen and ProfileScreen received professional redesigns with gradient treatments, improved hierarchy, and rich information architecture including privacy policy links and "Powered by BahariLink" attribution. The BahariMRV logo was redesigned with three ocean waves and a verification checkmark representing conservation and MRV principles.

**Watch for:** Placeholder data throughout (confirmed) — wallet revenue streams, verification pipeline stage counts, and retirement allocation display all use hardcoded mock values instead of pulling from DTOs. This creates visual fidelity to wireframes but no functional integration with backend data. GPS badge placement (confirmed) — moved from GPS section to Plot section header, which clusters GPS context but distances the badge from the actual GPS capture button. Evidence count formatting (likely) — uses bullet separator but aggregates photo and audio into one StatusChip instead of separate indicators per wireframe. Typography sizing (confirmed) — metrics use headlineMedium but some labels remain larger than wireframe's compact treatment.

**Verdict**: NEEDS_CHANGES

## High-level view

All three wireframe-targeted screens implement the specified layouts with new design system components, but placeholder data disconnects visual design from functional state. The wallet preview on HomeScreen shows three hardcoded revenue streams with emerald amounts ("$1,214", "$8,900", "$6,068") instead of pulling from DashboardDto fields that don't yet exist, creating a visual mockup that can't reflect actual project finances. The verification pipeline on CreditsScreen displays four numbered stages with checkmark logic for completed stages, but stage detail text ("Plot records · monitors") lacks the explicit counts shown in wireframe C3 ("316 plot records · 14 monitors") because no pipeline data structure provides them. The retirement certificate shows a hardcoded allocation line ("community 60 · project 25 · platform 15 🔒") that should derive from RetirementResultDto or RevenueRuleDto fields.

GPS badge placement creates a usability tradeoff: relocating "GPS locked ±18 m" from the GPS location card to the Plot & methodology card header improves visual grouping with plot context per wireframe C2, but now the badge sits two cards away from the "Use device GPS" button that determines its state, requiring users to scroll between the badge and the action that updates it. Evidence count formatting in MonitorScreen consolidates photo and audio counts into a single StatusChip ("Geo-tagged photos · 3 · Voice · 1") rather than separate indicators, which matches the wireframe's bullet-separated format but may reduce scannability compared to discrete badges.

Typography improvements are partially complete: CompactMetricCard on HomeScreen now uses headlineMedium for metric values and Bold weight, creating the large-number visual hierarchy specified in wireframe C1, but labels remain labelMedium where the wireframe shows even smaller sizing. The "Hash anchors to CreditMRV contract on sync" text in MonitorScreen received labelSmall styling with primary color and Medium weight per the "Outpost label" wireframe note.

Login and profile screens received professional Material 3 redesigns with gradient backgrounds, improved information architecture, and proper attribution. AuthScreen introduces a gradient background, professional logo placement, structured sign-in and registration forms with role selection chips, Jahazi SSO integration, security notices, and "Powered by BahariLink" footer. ProfileScreen adds a gradient header with avatar, quick stats card showing identity status/platforms/capabilities, enhanced identity and linked platforms cards, a settings section with privacy policy and terms links, app info card, and the BahariLink attribution.

The BahariMRV logo redesign produces a professional identity: three flowing ocean wave curves in emerald and blue representing conservation, overlaid with a navy checkmark in the top-right representing verification, creating a cohesive symbol for the blue carbon MRV platform. A simplified icon version condenses the wave-and-check motif for navigation bars and small displays.

<details>
<summary>Issues (8)</summary>

1. **Wallet revenue streams use hardcoded data** — DashboardDto lacks revenue fields (tourismContributions, carbonCreditSales, communityShare). Add these fields to the DTO and update DashboardContent to conditionally render WalletRevenueStream only when data exists.
2. **Verification pipeline stage details lack explicit counts** — wireframe C3 shows "316 plot records · 14 monitors" but current implementation shows generic "Plot records · monitors". Backend should provide PipelineStageDto with plotRecordCount and monitorCount fields.
3. **Retirement allocation is hardcoded** — "community 60 · project 25 · platform 15 🔒" should derive from RetirementResultDto.allocation or RevenueRuleDto.allocations field structure.
4. **GPS badge distance from GPS capture action** — badge moved to Plot section header (two cards up from GPS location card), creating spatial gap between state indicator and the button that updates it. Consider duplicating the badge or restoring it to GPS card.
5. **Evidence count format aggregates indicators** — wireframe C2 may intend separate badges for "Geo-tagged photos · 3" and "Voice · 1" rather than one combined StatusChip. Clarify wireframe intent or split into separate indicators.
6. **Metric label typography not fully compact** — CompactMetricCard uses labelMedium for labels where wireframe C1 suggests smaller sizing. Test labelSmall or reduce font scale.
7. **NDVI status badge lacks legend integration** — HomeScreen adds NDVI StatusChip above map but wireframe C1 may intend a legend overlay on the map surface itself. Clarify whether current placement matches intent.
8. **Profile screen lacks actual avatar image support** — ProfileHeader shows a placeholder icon but no mechanism to upload or display user photos. Consider adding avatar upload if this is a future requirement.

</details>

<details>
<summary>Details</summary>

## Design system enhancements match wireframe requirements

Three new composables in Common.kt fulfill wireframe-specified visual treatments. VerifiedBadge renders a 20.dp Surface with primary color background, extraSmall shape, and white checkmark icon. StageNumberCircle displays a 40.dp Surface with conditional styling: completed stages show primary background with white checkmark icon, active stages show surfaceVariant background with the stage number. WalletRevenueStream creates a SpaceBetween row with label in bodyMedium and amount in headlineSmall Bold with primary color.

The formatPrice helper function takes AmountDto, strips trailing zeros, and appends " 📦" to create the marketplace price format ("$28 📦"), though the package emoji may render inconsistently across Android versions.

## HomeScreen implements C1 wireframe layout with placeholder wallet data

DashboardContent restructures the metrics section to enforce a 2x2 grid for the first four metrics using `dashboard.metrics.take(4).chunked(2)`, wrapping each pair in a Row with equal weight, while additional metrics remain full-width. CompactMetricCard received typography adjustments: metric value uses headlineMedium with Bold weight, and the Row containing value and unit aligns to Bottom baseline. Labels remain labelMedium, which may be larger than the wireframe's compact treatment.

The NDVI status display moved above the map as a Row containing StatusChip("NDVI: ${it.status}") followed by optional source and acquisition timestamp metadata. This separates the status from the map legend text inside ProjectMap, which explains the visualization but doesn't provide status.

The wallet section introduces a LedgerCard with SectionHeader("Project wallet"), explanatory text ("Revenue streams and community share"), and three WalletRevenueStream composables displaying hardcoded values: "Tourism contributions" / "$1,214", "Carbon credit sales" / "$8,900", "Community share (60%)" / "$6,068". A TODO comment acknowledges "Replace with real wallet data when DashboardDto includes revenue fields" — the backend DTO currently has no tourismContributions, carbonCreditSales, or communityShare fields, so this implementation creates visual fidelity to wireframe C1 but zero functional integration.

Alert cards add severity indicators as colored Circle icons (12.dp size) in error/tertiary/primary colors based on alert.severity, positioned before the alert title and detail text.

```kotlin
// Wireframe C1: Wallet section (placeholder implementation)
LedgerCard {
    SectionHeader("Project wallet")
    Text("Revenue streams and community share", /* ... */)
    // TODO: Replace with real wallet data when DashboardDto includes revenue fields
    WalletRevenueStream("Tourism contributions", "$1,214")
    WalletRevenueStream("Carbon credit sales", "$8,900")
    WalletRevenueStream("Community share (60%)", "$6,068")
}
```

## MonitorScreen implements C2 wireframe with GPS badge repositioning

The header Row displays queue count as StatusChip with WARNING tone when pendingCount > 0 or POSITIVE tone "Synced" when zero, replacing the previous text-only indicator.

The GPS locked badge moved from the GPS location card to the Plot & methodology card header, appearing inline with "Plot & methodology" text when coordinates are present: `StatusChip("GPS locked ±${state.gpsAccuracy.ifBlank { "?" }} m", tone = ChipTone.POSITIVE)`. This clusters GPS context with plot metadata per a literal reading of wireframe C2's badge placement, but creates a spatial gap: the badge sits two cards above the "Use device GPS" button, requiring users to scroll between the indicator and the action that updates it. The GPS location card retains its button and coordinate fields but loses its visual status indicator.

Evidence count formatting implements the wireframe C2 "Geo-tagged photos · N screen" format by constructing a StatusChip with bullet-separated parts when evidence exists:

```kotlin
if (state.evidence.isNotEmpty()) {
    val photoCount = state.evidence.count { it.kind == EvidenceKind.PHOTO }
    val audioCount = state.evidence.count { it.kind == EvidenceKind.VOICE }
    val parts = mutableListOf<String>()
    if (photoCount > 0) parts.add("Geo-tagged photos · $photoCount")
    if (audioCount > 0) parts.add("Voice · $audioCount")
    StatusChip(parts.joinToString(" · "), tone = ChipTone.POSITIVE)
}
```

This aggregates photo and audio into one StatusChip ("Geo-tagged photos · 3 · Voice · 1") rather than separate indicators. The wireframe shows "Geo-tagged photos · 3 screen" and a voice button with microphone icon, which could be interpreted as one badge with both counts or two separate badges.

The voice recording button adds a Mic icon before the text, and the "Hash anchors to CreditMRV contract on sync" text receives labelSmall styling with primary color and Medium weight.

## CreditsScreen implements C3 and C4 wireframes with partial data integration

VerificationStage composable redesign replaces the basic Surface with a Row containing StageNumberCircle, stage label and detail Column, and StatusChip. The isCompleted logic checks `status.uppercase().contains("COMPLETE") || status.uppercase().contains("VERIFIED")`, which may miss edge cases like "VERIFICATION_COMPLETE" or "VERIFIED_EXTERNAL".

Stage detail text format ("Plot records · monitors", "NDVI · Sentinel-2 tile matched") matches wireframe C3's bullet-separated structure but lacks the explicit counts shown in the wireframe ("316 plot records · 14 monitors"). A TODO comment in the CreditsScreen acknowledges "Replace detail text with actual counts when pipeline data structure provides them", indicating the backend doesn't yet return PipelineStageDto with plotRecordCount and monitorCount fields.

```kotlin
// Wireframe C3: Stage detail text (count data missing)
VerificationStage(
    number = "1",
    label = "Field data complete",
    detail = "Plot records · monitors",  // Should be "316 plot records · 14 monitors"
    status = "PENDING",
    isActive = false
)
```

DocumentItem composable adds IPFS hash display with monospace font ("ipfs://Qm...") when documents have IPFS references, though the current implementation shows the hardcoded text "ipfs://Qm..." for all documents since no IPFS hash field exists in the document data structure.

MarketplaceListingCard adds VerifiedBadge next to the project name when `listing.registry.status.uppercase().contains("VERIFIED") || listing.registry.status.uppercase().contains("ACTIVE")`. The card uses formatPrice(listing.pricePerTonne) to display the price in headlineMedium Bold with primary color, creating the prominent "$28 📦" format. SDG co-benefits display as text "🌍🌊🌿 SDG co-benefits" with emoji icons when listing.coBenefits is not empty.

RetirementReceipt received formatting enhancements: the retired quantity displays in headlineMedium Bold with primary color ("100 tCO₂e retired"), a "Dual retirement record" section header explains the registry-and-chain approach, and reference displays use monospace font with truncation. The allocation display shows hardcoded text "community 60 · project 25 · platform 15 🔒" with a TODO comment: "Replace with data-driven allocation when RetirementResultDto includes breakdown".

## Login and profile screens receive professional Material 3 redesigns

AuthScreen replaces the basic form with a gradient background, professional logo placement (BahariMrvLogo at 80.dp), app title in displaySmall, tagline in a primaryContainer pill, and "Powered by BahariLink" footer. The sign-in form moves into a Card with elevation and border, structured with headline, descriptive text, email and password fields with proper keyboard types, a 52.dp Button with lock icon and loading state, an OR divider, Jahazi SSO button, and a registration link. The registration form adds displayName, phone, role selection with FlowRow of RoleChip toggles, and identity provisioning notice.

PasswordField introduces a visibility toggle IconButton with VisibilityOff/Visibility icons.

ProfileScreen implements a rich information architecture:

- **ProfileHeader**: Card with horizontal gradient background (primaryContainer to secondaryContainer), 72.dp avatar Surface with gradient border and AccountCircle icon, user display name in titleLarge Bold, role badge in a colored pill, and issuer text.
- **QuickStatsCard**: Three-column stats display with vertical dividers showing identity status (with color-coded value), linked platforms count, and role capabilities count, each with icon, value, and label.
- **EnhancedIdentityCard**: IOTA identity details with Lock icon header, network badge with Verified icon, DID and wallet address with monospace font, security custody notice in a surfaceVariant pill, and retry provisioning button for non-active identities. Role capabilities display below as separate RoleCapabilityCard instances.
- **EnhancedLinkCard**: Linked platform cards with Link icon, platform name, issuer, linked timestamp, and status chip.
- **SettingsSection**: Card with three clickable rows (Privacy Policy, Terms of Service, About BahariMRV) each with icon, title, subtitle, and chevron-right indicator. Privacy and terms links open browser intents to baharilink.co.ke URLs.
- **AppInfoCard**: Centered display with BahariMrvIcon, app name, description ("Conservation Operations & Verification"), version with BuildConfig, and platform description ("IOTA Testnet • Field Evidence • Independent Verification").
- **Sign out button**: Full-width 52.dp Button with error color, ExitToApp icon, and SemiBold text.
- **BahariLink attribution**: Row with BahariMrvIcon and "Powered by BahariLink" text.

## BahariMRV logo redesign creates professional identity

Logo.kt replaces the previous logo with a professional design featuring three flowing ocean wave curves and a verification checkmark. BahariMrvLogo composable draws three cubic bezier paths representing ocean waves at different y-offsets with rounded stroke caps: top wave in accent blue, middle wave in lighter emerald (0.7 alpha), bottom wave in primary emerald. A checkmark path in navy overlays the top-right position.

BahariMrvIcon provides a simplified version for navigation bars: a single wave curve with checkmark overlay, both drawn in the provided color, at reduced stroke widths.

</details>

<details>
<summary>File map</summary>

**Modified files:**

- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt` — Added VerifiedBadge, StageNumberCircle, WalletRevenueStream composables and formatPrice helper
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/home/HomeScreen.kt` — Implemented 2x2 metrics grid, wallet preview section, alert severity indicators, NDVI status badge placement, CompactMetricCard typography adjustments
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/monitor/MonitorScreen.kt` — Repositioned GPS badge to Plot section, formatted evidence count with bullets, added microphone icon to voice button, styled hash anchor text as Outpost label
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/credits/CreditsScreen.kt` — Redesigned VerificationStage with numbered circles and checkmarks, added VerifiedBadge to marketplace listings, formatted price with emoji, displayed SDG co-benefits as emojis, enhanced RetirementReceipt formatting with allocation display
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/auth/AuthScreen.kt` — Professional redesign with gradient background, logo placement, structured forms, role selection, Jahazi SSO, security notices, BahariLink attribution
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/profile/ProfileScreen.kt` — Added gradient header with avatar, quick stats card, enhanced identity and platform cards, settings section with privacy/terms links, app info card, sign-out button, BahariLink attribution
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Logo.kt` — Professional logo redesign with three ocean waves and verification checkmark, simplified icon version

**No changes to:**
- Any ViewModels (HomeViewModel, MonitorViewModel, CreditsViewModel, ProfileViewModel, AuthViewModel)
- Any repositories or data sources
- Any DTOs or data models
- Theme.kt (colors already correct)
- Navigation graph
- Any backend code

</details>
