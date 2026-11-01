# BahariMRV Android UI Pixel-Perfect Redesign Plan

**Task:** Pixel-perfect UI redesign of Conservation Module screens C1-C4 to match wireframe specifications exactly  
**Date:** 2024  
**Scope:** HomeScreen, MonitorScreen, CreditsScreen, and shared components  
**Constraint:** NO functionality loss - ViewModels, data flow, and navigation remain unchanged

---

## Implementation Plan

### 1. Update Material3 Typography System to Support Wireframe Specifications

**Rationale:** The wireframes specify exact typography scales (displayLarge 57sp, displayMedium 45sp, displaySmall 36sp, labelSmall 11sp) that must be available in the theme. The current implementation uses only headlineMedium and titleLarge, which don't match the wireframe's large metric displays.

**Files:**
- Create: `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Type.kt`
- Modify: `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt`

**Changes:**
- Create Type.kt with custom Typography configuration:
  - displayLarge: 57sp, Bold (for C4 price display "$28 📦")
  - displayMedium: 45sp, Bold (for C1 metric values)
  - displaySmall: 36sp, Bold (for C2 survival counts)
  - headlineMedium: 34sp, Bold (current, keep for page titles)
  - titleLarge: 22sp, SemiBold (current, keep)
  - titleMedium: 16sp, SemiBold (current, keep)
  - labelSmall: 11sp, SemiBold (for C1 metric labels)
  - bodyMedium: 14sp, Normal (current, keep)
- Import and apply the Typography to BahariMrvTheme in Theme.kt

**Verify:** Build the app with `./gradlew assembleDebug` and confirm no compilation errors

---

### 2. C1 HomeScreen: Dashboard Metrics with Large Typography

**Rationale:** Wireframe C1 shows metric values in VERY LARGE type (45-57sp) with small labels (11sp) underneath. Current implementation uses headlineMedium (~34sp) which is too small. Wallet amounts must be right-aligned in emerald green (#079B76).

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/home/HomeScreen.kt`

**Changes:**

**2a. Metric Card Typography Update:**
- In `CompactMetricCard`:
  - Change metric value from `MaterialTheme.typography.headlineMedium` to `MaterialTheme.typography.displayMedium`
  - Change `fontWeight = FontWeight.Bold` (keep)
  - Change metric label from `MaterialTheme.typography.labelMedium` to `MaterialTheme.typography.labelSmall`
- In `MetricCard` (full-width version):
  - Change value display from `MaterialTheme.typography.titleLarge` to `MaterialTheme.typography.displayMedium`

**2b. Map Section Height Reduction:**
- In `ProjectMap` function:
  - Change Canvas height from `height(220.dp)` to `height(200.dp)` to match C1 compact map

**2c. Wallet Revenue Stream Alignment:**
- In `WalletRevenueStream` component (in Common.kt, but called from HomeScreen):
  - Already has emerald green color for amount via `MaterialTheme.colorScheme.primary`
  - Change alignment to ensure amount is right-aligned (already implemented correctly)
  - Verify amount uses `headlineSmall` (already correct per wireframe)

**2d. Alert Badge Red Dot:**
- In dashboard alerts section, change Circle icon:
  - Current size: 12.dp (matches C1 - keep)
  - Verify error color is used for critical alerts (already correct)

**2e. Action Buttons Side-by-Side Equal Width:**
- Already implemented correctly in Row with `Modifier.weight(1f)` on each button
- Verify buttons are in a Row with equal weights (already correct)

**Verify:** 
- Run `./gradlew assembleDebug`
- Launch app and navigate to Home screen
- Visually compare metric value sizes - should be significantly larger (45sp vs previous 34sp)
- Verify metric labels are smaller (11sp)
- Verify wallet amounts are right-aligned and green

---

### 3. C2 MonitorScreen: GPS Badge Inline and Survival Layout

**Rationale:** Wireframe C2 shows GPS badge INLINE with plot title (not a separate section), survival counts in side-by-side two-column layout with LARGE numbers (36sp), and queue badge in top-right header with yellow/amber background.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/monitor/MonitorScreen.kt`

**Changes:**

**3a. GPS Badge Position:**
- In "Plot & methodology" LedgerCard section:
  - GPS badge is already positioned inline with title in a Row - verify this is correct
  - Badge shows "GPS locked ±X m" when coordinates exist
  - This already matches C2 wireframe - confirm visually

**3b. Queue Badge in Header:**
- In header Row at top of screen:
  - Already shows StatusChip with queue count in top-right position
  - Uses ChipTone.WARNING (yellow/amber) when items queued - matches C2
  - Position and styling already correct - verify visually

**3c. Survival Count Large Numbers:**
- In "Survival measurement" LedgerCard:
  - Currently uses two OutlinedTextField inputs in a Row (Planted and Alive)
  - This is CORRECT for input fields - they should remain as text fields
  - The wireframe shows large numbers (36sp) for the DISPLAY of captured data in submission history
  - In submission history item cards, the display line shows: `"${item.record.aliveCount}/${item.record.plantedCount} alive"`
  - Change this text style from `MaterialTheme.typography.bodyMedium` to custom display
  - Create a Row with two columns showing counts in `displaySmall` (36sp)
  - Format: "12 alive" | "15 planted" side by side

**3d. Evidence Section Label (Not Card):**
- "Evidence manifest" section already uses text labels with icons
- NOT wrapped in extra card decoration (already correct)
- Evidence items show filename + kind + status (already correct)
- Verify this matches C2 wireframe

**3e. Hash Anchor Label Styling:**
- Text "Hash anchors to CreditMRV contract on sync":
  - Change from `MaterialTheme.typography.labelSmall` (already correct size)
  - Verify green color: `color = MaterialTheme.colorScheme.primary` (already correct)
  - Verify `fontWeight = FontWeight.Medium` (already set)
  - This already matches C2 - confirm visually

**Verify:**
- Run `./gradlew assembleDebug`
- Navigate to Monitor screen
- Verify GPS badge appears inline with "Plot & methodology" title
- Verify queue badge in top-right with amber background
- Capture and save a monitoring record, then check submission history shows large numbers (36sp) for alive/planted counts

---

### 4. C3 CreditsScreen: Verification Pipeline with Checkmarks

**Rationale:** Wireframe C3 shows green checkmarks (✓) NOT circles for completed stages, yellow info icon (ⓘ) for pending stages, and specific stage layout [Icon] [Title + Detail] [Status Badge]. Documents section shows IPFS hashes.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/credits/CreditsScreen.kt`
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt`

**Changes:**

**4a. Stage Icon Update (Common.kt):**
- In `StageNumberCircle` composable:
  - Completed stages currently show green circle with white checkmark (✓) - this is CORRECT
  - Pending stages show circle with number - CHANGE to show yellow info icon (ⓘ)
  - Add parameter `isPending: Boolean = false`
  - When pending: show yellow/amber background with info icon instead of number
  - When neither completed nor pending (todo): show gray circle with number (keep existing)

**4b. Stage Layout (CreditsScreen.kt):**
- In `VerificationStage` composable:
  - Current layout already matches: [StageNumberCircle] [Column with title+detail] [StatusChip]
  - This is CORRECT per C3 wireframe
  - Verify icon size is appropriate (40.dp currently - matches wireframe)
  - Update StageNumberCircle calls to pass `isPending = status.uppercase() == "PENDING"`

**4c. Document IPFS Hashes:**
- In `DocumentItem` composable:
  - Already shows "ipfs://Qm..." in monospace font
  - Already shows document name and status chip
  - This matches C3 wireframe - verify visually

**Verify:**
- Run `./gradlew assembleDebug`
- Navigate to Credits screen
- Verify completed stages show green circle with white checkmark (✓)
- Verify pending stages show yellow circle with info icon (ⓘ)
- Verify documents section shows IPFS hashes in monospace

---

### 5. C4 CreditsScreen: Marketplace Price Display and Retirement Certificate

**Rationale:** Wireframe C4 shows price in VERY LARGE type (57sp) with emoji "$28 📦", "SDG co-benefits ✓" as small green badge (NOT large), retirement certificate with dark navy background (#12263B), and allocation display "community 60 · project 25 · platform 15 🔒" at bottom.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/credits/CreditsScreen.kt`
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt`

**Changes:**

**5a. Price Display Typography:**
- In `MarketplaceListingCard`:
  - Change price Text from `MaterialTheme.typography.headlineMedium` to `MaterialTheme.typography.displayLarge`
  - Keep `fontWeight = FontWeight.Bold`
  - Keep `color = MaterialTheme.colorScheme.primary` (emerald green)
  - The `formatPrice()` function already adds emoji "📦" (correct)

**5b. SDG Co-Benefits Badge Size:**
- In `MarketplaceListingCard`:
  - Currently shows: `Text("🌍🌊🌿 SDG co-benefits", style = MaterialTheme.typography.bodySmall, color = primary)`
  - This is already small and green - CORRECT per C4 wireframe
  - Verify it's not displayed as a large badge (already correct)

**5c. Retirement Certificate Navy Background:**
- In `RetirementReceipt` composable:
  - Currently uses `LedgerCard` which has white background
  - CHANGE: Add optional `backgroundColor` parameter to LedgerCard, or
  - Create new `NavyLedgerCard` composable in Common.kt:
    - Same as LedgerCard but with `backgroundColor = Color(0xFF12263B)` (navy)
    - Text color: `contentColor = Color(0xFFDFF2FB)` (light blue)
  - Wrap entire RetirementReceipt content in NavyLedgerCard
  - Update text colors to use light colors on dark background

**5d. Quantity Display in Retirement:**
- In `RetirementReceipt`:
  - Currently shows `"${result.quantityTonnes} tCO₂e retired"` in headlineMedium
  - KEEP this size - it's appropriate for the certificate
  - Verify primary color (emerald) is used

**5e. Allocation Display at Bottom:**
- In `RetirementReceipt`:
  - Already shows: `"community 60 · project 25 · platform 15 🔒"`
  - Already uses `bodySmall` and `onSurfaceVariant` color
  - This is CORRECT per C4 wireframe
  - When inside NavyLedgerCard, update color to light variant

**Verify:**
- Run `./gradlew assembleDebug`
- Navigate to Credits screen marketplace
- Verify price displays in very large type (57sp) with emoji
- Verify SDG co-benefits is small green text (not large badge)
- Purchase and retire credits to see retirement certificate
- Verify retirement certificate has dark navy background (#12263B)
- Verify allocation display appears at bottom with lock emoji

---

### 6. Common Components: Navy Card and Enhanced Formatting

**Rationale:** Multiple screens need access to the navy-background card variant for financial/proof displays per C4 wireframe. Typography helpers need to support the new display sizes.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt`

**Changes:**

**6a. Add NavyLedgerCard Composable:**
```kotlin
@Composable
fun NavyLedgerCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Card(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large, // 18dp per DESIGN.md
        colors = CardDefaults.cardColors(
            containerColor = Color(0xFF12263B), // navy
            contentColor = Color(0xFFDFF2FB)  // light blue
        ),
        border = BorderStroke(1.dp, Color(0xFF2A4356)),
    ) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp), content = content)
    }
}
```

**6b. Update StageNumberCircle for Pending State:**
- Add `isPending: Boolean = false` parameter
- When `isPending`:
  - Background: `MaterialTheme.colorScheme.tertiaryContainer` (amber/yellow tint)
  - Show info icon "ⓘ" instead of number
  - Content color: `MaterialTheme.colorScheme.onTertiaryContainer`

**6c. Update VerifiedBadge Size (If Needed):**
- Current size is 20.dp with 14.dp icon
- Verify this matches wireframe (appears correct)
- Keep current implementation

**Verify:**
- Run `./gradlew assembleDebug`
- Confirm no compilation errors
- All screens that use updated components compile successfully

---

### 7. Theme Color Verification Against DESIGN.md

**Rationale:** The DESIGN.md specifies exact colors that must be used: primary emerald #079B76 (not #08775E currently in Theme.kt), navy #12263B, etc. Verify all colors match the design system.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt`

**Changes:**

**7a. Primary Color Update:**
- Current light primary: `Color(0xFF08775E)` (close but not exact)
- DESIGN.md primary: `#079b76` = `Color(0xFF079B76)`
- UPDATE primary color to match exactly
- Current primary (#08775E) is close enough that this is LOW PRIORITY
- Document this as "close match" rather than changing if it causes visual disruption

**7b. Navy Color Verification:**
- DESIGN.md navy: `#12263b` = `Color(0xFF12263B)`
- Current tertiary color: `Color(0xFF12263B)` - CORRECT
- No change needed

**7c. Canvas/Background Color:**
- DESIGN.md canvas: `#f4f7f8` = `Color(0xFFF4F7F8)`
- Current background: `Color(0xFFF4F7F8)` - CORRECT
- No change needed

**Verify:**
- Optionally update primary from #08775E to #079B76 if visual consistency is critical
- Most colors already match DESIGN.md
- Build and visually compare with wireframe

---

### 8. Final Typography Scale Configuration

**Rationale:** Ensure all typography scales from DESIGN.md are available and Material3 theme uses them consistently.

**Files:**
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Type.kt` (new)
- `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt`

**Changes:**

**8a. Create Type.kt with full Material3 Typography:**
```kotlin
package ke.co.baharilink.baharimrv.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

val BahariTypography = Typography(
    // C4 Price: $28 📦 in very large display
    displayLarge = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Bold,
        fontSize = 57.sp,
        lineHeight = 64.sp,
        letterSpacing = (-0.25).sp
    ),
    // C1 Metrics: large values 45sp
    displayMedium = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Bold,
        fontSize = 45.sp,
        lineHeight = 52.sp,
        letterSpacing = 0.sp
    ),
    // C2 Survival counts: 36sp side-by-side
    displaySmall = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Bold,
        fontSize = 36.sp,
        lineHeight = 44.sp,
        letterSpacing = 0.sp
    ),
    // Page titles: keep existing 34sp
    headlineMedium = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Bold,
        fontSize = 34.sp,
        lineHeight = 40.sp,
        letterSpacing = 0.sp
    ),
    // Section headings: keep existing
    titleLarge = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.SemiBold,
        fontSize = 22.sp,
        lineHeight = 28.sp,
        letterSpacing = 0.sp
    ),
    titleMedium = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.SemiBold,
        fontSize = 16.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.15.sp
    ),
    // C1 Metric labels: 11sp small labels
    labelSmall = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.SemiBold,
        fontSize = 11.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.5.sp
    ),
    labelMedium = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.SemiBold,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.5.sp
    ),
    // Body text: keep existing 14sp
    bodyMedium = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Normal,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.25.sp
    ),
    bodySmall = TextStyle(
        fontFamily = FontFamily.Default,
        fontWeight = FontWeight.Normal,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.4.sp
    ),
)
```

**8b. Import and Use in Theme.kt:**
- Import Typography configuration
- Pass `typography = BahariTypography` to MaterialTheme()

**Verify:**
- Run `./gradlew assembleDebug`
- All text displays at correct sizes per wireframe
- No compilation errors

---

## Verification Strategy

### Build Verification
```bash
cd /Users/Komarov/Documents/BahariMRV/android
./gradlew clean
./gradlew assembleDebug
```
Expected: Build succeeds with no errors

### Screen-by-Screen Visual Verification

**C1 Home Screen:**
1. Launch app and navigate to Home
2. Load a project dashboard
3. Verify metric values are LARGE (45sp) and bold
4. Verify metric labels are small (11sp)
5. Verify wallet amounts right-aligned in emerald green
6. Verify map is ~200dp height (compact)
7. Verify alert dots are 12dp red circles
8. Verify action buttons side-by-side equal width

**C2 Monitor Screen:**
1. Navigate to Monitor screen
2. Verify GPS badge appears inline with "Plot & methodology" title (not separate section)
3. Verify queue badge in top-right header with amber background when items queued
4. Capture a monitoring record
5. View submission history
6. Verify alive/planted counts display in large numbers (36sp) in two-column layout
7. Verify evidence section uses text labels (not card wrapper)
8. Verify "Hash anchors..." text is small green label at bottom

**C3 Credits Screen - Verification Pipeline:**
1. Navigate to Credits screen
2. Scroll to Verification Pipeline section
3. Verify completed stages show green circle with white checkmark ✓
4. Verify pending stages show yellow/amber circle with info icon ⓘ
5. Verify stage layout: [Icon] [Title+Detail column] [Status badge]
6. Verify documents section shows IPFS hashes in monospace font

**C4 Credits Screen - Marketplace:**
1. Scroll to marketplace listings
2. Verify price displays in VERY LARGE type (57sp) with emoji "$28 📦"
3. Verify "SDG co-benefits" is small green text (not large badge)
4. Verify available tonnes and registry info displayed
5. Purchase and retire credits to generate certificate
6. Verify retirement certificate has dark navy background (#12263B)
7. Verify text on navy background is light blue (#DFF2FB)
8. Verify allocation display at bottom: "community 60 · project 25 · platform 15 🔒"

### Functionality Verification

**NO functionality should be lost:**
- [ ] ViewModels unchanged (no modifications to HomeViewModel, MonitorViewModel, CreditsViewModel)
- [ ] Data flow unchanged (all state collection from ViewModels works)
- [ ] Navigation unchanged (bottom nav, screen transitions work)
- [ ] API calls unchanged (all network requests function)
- [ ] Database operations unchanged (Room queries, sync works)
- [ ] Location services unchanged (GPS capture works)
- [ ] Evidence upload unchanged (photos/audio attach and upload)
- [ ] Offline sync unchanged (WorkManager jobs run)

### Performance Verification
- [ ] App launches without delay
- [ ] Screen transitions are smooth
- [ ] Large typography doesn't cause layout performance issues
- [ ] No memory leaks from new composables

---

## Project Context

**Build System:** Gradle with Kotlin DSL, Android Gradle Plugin 8.7.3, Kotlin 2.1.0  
**Build Command:** `cd android && ./gradlew assembleDebug`  
**Test Command:** `cd android && ./gradlew test` (unit tests currently minimal)  
**Language:** Kotlin  
**UI Framework:** Jetpack Compose with Material3  
**Architecture:** MVVM with Clean Architecture, Hilt DI, Room database  

**Key Patterns from Codebase:**
- Composable screens consume state from ViewModels via `collectAsStateWithLifecycle()`
- UI components are in `/ui/components/Common.kt` for reuse
- Theme colors from Material3 color scheme, shapes from DESIGN.md (8dp, 12dp, 18dp)
- LedgerCard is standard container with white background and border
- StatusChip shows state with color-coded backgrounds (positive/warning/error/neutral)
- Typography uses Material3 default hierarchy, now extended with display sizes

**Contribution Requirements (from SPEC.md):**
- All monetary values use BigDecimal (never float)
- All timestamps are RFC3339 strings
- Evidence hashes computed before display
- IOTA references shown with network labels
- Sandbox/test modes explicitly labeled
- No hardcoded success states for external services

**Relevant Files:**
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/home/HomeScreen.kt` - C1 Dashboard
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/monitor/MonitorScreen.kt` - C2 Field Capture
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/credits/CreditsScreen.kt` - C3 Pipeline + C4 Marketplace
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt` - Shared components
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt` - Color scheme
- `/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Type.kt` - Typography (to be created)

**Directory Structure:**
```
android/app/src/main/java/ke/co/baharilink/baharimrv/
├── ui/
│   ├── home/
│   │   ├── HomeScreen.kt (C1)
│   │   └── HomeViewModel.kt (unchanged)
│   ├── monitor/
│   │   ├── MonitorScreen.kt (C2)
│   │   └── MonitorViewModel.kt (unchanged)
│   ├── credits/
│   │   ├── CreditsScreen.kt (C3 + C4)
│   │   └── CreditsViewModel.kt (unchanged)
│   ├── components/
│   │   └── Common.kt (shared UI components)
│   └── theme/
│       ├── Theme.kt (colors, shapes)
│       └── Type.kt (typography - new)
├── data/ (unchanged)
├── domain/ (unchanged)
└── MainActivity.kt (unchanged)
```

---

## Implementation Notes

1. **Typography First:** Create Type.kt and wire it into Theme.kt before modifying screens. This ensures display scales are available.

2. **Screen-by-Screen:** Implement changes one screen at a time (Home → Monitor → Credits) to isolate issues.

3. **Component Updates:** Modify Common.kt components (NavyLedgerCard, StageNumberCircle) before using them in screens.

4. **Incremental Verification:** Build and visually verify after each major change. Don't batch all changes before testing.

5. **No ViewModel Changes:** This is purely a UI presentation change. If logic changes are needed, the plan is wrong.

6. **Color Consistency:** Use MaterialTheme.colorScheme colors throughout. Only hardcode colors for exact wireframe specs (navy #12263B for retirement cert).

7. **Preserve Accessibility:** Large touch targets (48.dp), sufficient color contrast, semantic content descriptions already present - maintain these.

8. **Material3 Compliance:** All changes use Material3 components and conventions. No custom view hacks.

---

## Success Criteria

- [ ] All 8 plan items completed
- [ ] Build succeeds with no errors or warnings
- [ ] C1 metric values use 45sp typography
- [ ] C1 metric labels use 11sp typography
- [ ] C2 GPS badge inline with plot title
- [ ] C2 survival counts in submission history use 36sp in two columns
- [ ] C3 completed stages show green checkmark ✓
- [ ] C3 pending stages show yellow info icon ⓘ
- [ ] C4 price displays in 57sp with emoji
- [ ] C4 retirement certificate has navy background
- [ ] All functionality preserved (ViewModels, data flow, navigation unchanged)
- [ ] No performance regression
- [ ] Visual comparison with wireframe shows pixel-perfect match

---

**End of Implementation Plan**
