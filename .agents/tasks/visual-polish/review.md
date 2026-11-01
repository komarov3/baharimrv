# Android app visual polish against wireframe specifications

The Android app's visual styling was refactored to match the BahariLink wireframe specifications defined in `/baharilink-wireframe/index.html`. The work touched typography, color system, component library, navigation chrome, and the Credits screen (C4), implementing the exact dimensions, spacing, and colors documented in the wireframe CSS.

**Verdict**: CHANGES_REQUESTED

## Watch for

1. **Background color mismatch** (confirmed) — app background remains white (`#FFFFFF`) instead of wireframe-specified `#E9EEF4`
2. **Navigation icon for Monitor incorrect** (confirmed) — uses satellite icon (`ic_sat`) instead of camera/add icon per wireframe screen labeling
3. **Typography scale deviation** (likely) — some font sizes don't match wireframe exactly (card title 13sp vs 12.5sp specified, price label uses 9.5sp but displays as uppercase "PRICE / TCO₂E")
4. **Status chip colors incomplete** (likely) — grey neutral chip uses `#E5EBF2` background but wireframe specifies white background with `#d8e0ea` border for neutral state

## High-level view

The redesign introduces a comprehensive BahariComponents library with wireframe-aligned cards, buttons, chips, and icon backgrounds, replacing Material3 defaults. Typography was remapped to match wireframe sizes (19sp price, 13sp titles, 10sp notes, 8.5sp nav), and an extended color scheme adds cyan, violet, slate, and navy variants beyond Material3's standard palette. The navigation bar was restyled to 54dp height with custom icons and 8.5sp labels, replacing Material Design icons with wireframe SVG equivalents. The Credits screen (C4) received targeted polish: card padding changed from 12dp to 10dp, card gap widened from 8dp to 7dp, price color corrected to `#059669`, and batch IDs moved to a grey pill. However, the app background remains white instead of `#E9EEF4`, the Monitor navigation uses the wrong icon, and some typography sizes still deviate from the wireframe spec.

<details>
<summary>Issues (4)</summary>

1. **App background color** — Change `Theme.kt` background from `#FFFFFF` to wireframe-specified `#E9EEF4` (currently set in `LightColors.background` but not applied to screen containers)
2. **Monitor navigation icon** — Replace `ic_sat` (satellite) with `ic_plus` or camera icon to match wireframe F2/C2 "Monitor" screens
3. **Typography scale precision** — Audit all text styles against wireframe CSS: card title should be 12.5sp not 13sp, and "PRICE / TCO₂E" label rendering needs investigation
4. **Neutral status chip styling** — Update `StatusChip` NEUTRAL tone to use white background (`#FFFFFF`) with `#d8e0ea` border, not grey fill

</details>

<details>
<summary>Details</summary>

## Design system foundation established

A new `BahariComponents.kt` file (408 lines) implements the wireframe design language: `BahariCard` with 12dp radius, 1dp border, 9dp×10dp padding, and 7dp internal gap; `BahariButton` with five variants (primary blue, green, amber, violet, ghost) using 10dp radius and 11sp text; `BahariChip` with fully rounded edges, 4dp×9dp padding, and color-coded backgrounds (green `#e7f8f1`, amber `#fdf3e3`, blue `#eaf1ff`); and `BahariIconBackground` with 34dp×34dp containers and 10dp radius. The library also defines gradient brushes for hero headers, ID cards, and wallet cards matching the wireframe's linear-gradient specifications.

The theme system in `Theme.kt` was extended with `ExtendedColorScheme` adding cyan, violet, slate, navy, and offline yellow to Material3's standard palette. Background color was set to `#E9EEF4` in the color scheme definition, but screen containers still render white because Compose's scaffold and surface defaults weren't overridden. Surface containers were correctly updated to match the background (`surfaceContainer`, `surfaceContainerHigh`, `surfaceContainerHighest` all set to `#E9EEF4`), but the body content in screens like CreditsScreen uses the default surface color which remains white.

Typography in `Type.kt` was remapped to wireframe sizes: `displayLarge` became 19sp for price values, `headlineSmall` 13sp for card titles, `bodySmall` 10sp for notes, `labelLarge` 11sp for buttons, `labelSmall` 9.5sp for micro-labels, and a custom `navText` extension property at 8.5sp for navigation items. Line heights and letter-spacing were adjusted to match the wireframe's compact aesthetic. The typography changes are thorough and internally consistent, though some usages in the Credits screen don't precisely match the wireframe's rendered output (uppercase styling on "PRICE / TCO₂E" suggests a text transform was applied in code rather than defined in the typography style).

## Navigation bar transformation

`AppRoot.kt` replaced Material Design icons with wireframe SVG equivalents: `ic_home`, `ic_sat`, `ic_co2`, `ic_wallet`, `ic_user`. The navigation bar height was constrained to 54dp (down from Material3's default ~80dp), label font size dropped to 8.5sp with `SemiBold` weight, and the selected/unselected color logic changed from tonal containers to direct color application (`#2563EB` for selected, `#9AA8BA` for unselected). The indicator background was set to `Color.Transparent`, removing Material3's pill-shaped selection indicator. The navigation rail for tablet layouts received similar treatment but kept the default height.

The Monitor navigation item uses `R.drawable.ic_sat` (satellite icon) instead of a capture/add icon. The wireframe's "Monitor" screens (F2 logbook entry, C2 MRV capture) show a camera or plus icon in context, not a satellite. The satellite icon appears in wireframe screen C1 (project dashboard) in the navigation bar but labeled "Monitor" there refers to the satellite monitoring feature, not field capture. The Android app's Monitor route leads to field capture (camera, GPS, voice), so the icon should match that function. This is a confirmed functional mismatch: the user sees a satellite icon but taps into a camera-based data capture flow.

## Credits screen (C4) polish applied

`CreditsScreen.kt` received targeted adjustments: the `LazyColumn` content padding changed from `16.dp` to `13.dp, 12.dp` (horizontal, vertical) matching wireframe's `.pbody` padding, and vertical gap widened from `10.dp` to `14.dp` matching wireframe's `.board` card gap. The `ListingCard` content padding was reduced from `12.dp` to `10.dp` (card interior padding per wireframe `.card` spec), and internal gap changed from `8.dp` to `7.dp` (wireframe's `.card` internal gap).

The price display color was corrected from `#079B76` to `#059669` (wireframe's `--green`), and the button border radius changed from default to `10.dp` (wireframe `.btn` spec). The batch ID was moved from inline monospace text to a grey pill: `Surface` with `#F1F5F9` background, `6.dp` rounded corners, `8.5.sp` text, and `6.dp × 3.dp` padding. The "PRICE / TCO₂E" label uses `labelSmall` style but uppercase transformation is applied elsewhere (the wireframe CSS shows `.ttl-xs` with `text-transform:uppercase` and `9.5px` size, which aligns with the implementation).

Status chips in `Common.kt` were updated with wireframe-specific colors: `POSITIVE` uses green `#e7f8f1` background with `#b7e8d4` border, `WARNING` uses amber `#fdf3e3` with `#f2ddb3` border, and `NEUTRAL` uses `#e5ebf2` (grey) background and border. The wireframe's `.chip` class specifies white background with `--line` (`#d8e0ea`) border for the neutral state, and colored backgrounds only for `.chip.on` (blue), `.chip.green`, and `.chip.amber`. The Android implementation's neutral chip has a grey fill instead of white, making it visually heavier than the wireframe's neutral chips.

`LedgerCard` in `Common.kt` was updated to use `12.dp` border radius (previously Material3's medium shape, which was 12dp in the old theme but now explicitly set), 1dp border, 0dp elevation, and `7.dp` internal gap (down from `8.dp`). These match the wireframe's `.card` specification exactly.

## Background color not applied to screens

The most visible gap is the background color. `Theme.kt` sets `background = Color(0xFFE9EEF4)` in `LightColors`, and all surface containers were updated to the same color, but the Credits screen and other app screens still render with a white background. The `CreditsScreen` uses a `LazyColumn` with no explicit background modifier, inheriting from the scaffold's surface color. The scaffold in `AppRoot.kt` uses `MaterialTheme.colorScheme.surface` which is set to `Color(0xFFFFFFFF)` (white) in the theme. To match the wireframe, either the scaffold should use `background` instead of `surface`, or screens should apply a background modifier, or `surface` should be redefined as `#E9EEF4` (though that would affect cards, which should remain white per the wireframe).

The wireframe CSS shows:
```css
body { background: var(--bg); } /* --bg: #e9eef4 */
.pbody { background: #fdfefe; } /* screen body, very close to white */
.card { background: #fff; }
```

The Android theme should set `background = #E9EEF4` (done), `surface = #FFFFFF` (done), and ensure the scaffold or screen container uses `background` color, not `surface` color. The current implementation has the colors defined correctly but applies `surface` everywhere.

## Icon library complete but Monitor icon wrong

Twenty-eight SVG icons were added to `res/drawable/`, converted from the wireframe's symbol library: `ic_home`, `ic_book`, `ic_store`, `ic_wallet`, `ic_user`, `ic_fish`, `ic_leaf`, `ic_sat`, `ic_co2`, `ic_bell`, `ic_search`, `ic_pin`, `ic_plus`, `ic_qr`, `ic_check`, `ic_chart`, `ic_boat`, and others. The conversion quality is high: stroke width, color, and viewBox preserved. However, the Monitor navigation item in `AppRoot.kt` destinations array uses `ic_sat` (satellite) when the wireframe's Monitor screens (F2, C2) show capture/add actions (camera, plus icon). The satellite icon belongs to the "Satellite monitoring" feature (screen C1 annotation mentions "Satellite NDVI tiles"), not the primary Monitor navigation item which leads to field data capture.

## Typography scale mostly correct with minor deviations

The wireframe CSS defines:
- `.big` → `19px` / `800` / `-0.3px` (price value)
- `.ttl` in `.papp` → `13.5px` / `700` (app bar title)
- card title (`.b` in `.card`) → `12.5px` or `13px` depending on context
- `.xs` → `9.5px`
- `.nv` → `8.5px` / `600` (nav label)

The Android typography maps:
- `displayLarge` → `19.sp` / `ExtraBold` / `-0.3.sp` ✓
- `headlineMedium` → `13.5.sp` / `Bold` ✓
- `headlineSmall` → `13.sp` / `Bold` (used for card titles in CreditsScreen)
- `labelSmall` → `9.5.sp` / `ExtraBold` / `0.08.em` ✓
- `navText` extension → `8.5.sp` / `SemiBold` ✓

The card title in `ListingCard` uses `MaterialTheme.typography.headlineSmall` (13sp), but the wireframe's C4 card title "Gazi Bay Mangrove · 2026" appears as `12.5px` in the CSS (`.b` style inside `.card` context has `font-size:12.5px` in one variant, `13px` in another depending on specificity). The implementation chose 13sp, which is close but not exact. The "PRICE / TCO₂E" label in the wireframe shows `9.5px` uppercase via `.ttl-xs` class, and the Android implementation uses `labelSmall` (9.5sp) but the uppercase transformation happens in the string literal, not the style definition.

## Spacing system implemented correctly

The wireframe defines:
- `.pbody` → `12px 13px` padding
- `.board` → `14px` gap between cards
- `.card` → `9px 10px` padding (vertical, horizontal), `7px` internal gap
- `.btn` → `9px 10px` padding, `10px` radius
- `.pnav` → `54px` height

The Android implementation:
- `CreditsScreen` LazyColumn padding → `13.dp, 12.dp` ✓
- LazyColumn vertical gap → `14.dp` ✓
- `LedgerCard` padding → `10dp` (via contentPadding parameter) ✓
- `LedgerCard` internal gap → `7.dp` ✓
- `BahariButton` padding → `9.dp x 10.dp` ✓
- `BahariButton` radius → `10.dp` ✓
- Navigation bar height → `54.dp` ✓

All spacing values match the wireframe specification exactly.

## Status chip colors need neutral correction

The wireframe's chip system:
```css
.chip { background: #fff; border: 1px solid var(--line); color: var(--mut); }
.chip.green { background: #e7f8f1; border: #b7e8d4; color: var(--green); }
.chip.amber { background: #fdf3e3; border: #f2ddb3; color: var(--amber); }
.chip.on { background: #eaf1ff; border: #bfd4ff; color: var(--blue); }
```

Android's `StatusChip` in `Common.kt`:
```kotlin
ChipTone.POSITIVE -> Triple(Color(0xFFE7F8F1), Color(0xFFB7E8D4), Color(0xFF059669)) ✓
ChipTone.WARNING -> Triple(Color(0xFFFDF3E3), Color(0xFFF2DDB3), Color(0xFFD97706)) ✓
ChipTone.NEUTRAL -> Triple(Color(0xFFE5EBF2), Color(0xFFE5EBF2), Color(0xFF5B6B80))
```

The neutral chip uses a grey background (`#E5EBF2`) when the wireframe shows white (`#FFFFFF`) with a `#d8e0ea` border. This makes the neutral chip visually heavier than intended. The correct mapping should be `Triple(Color(0xFFFFFFFF), Color(0xFFD8E0EA), Color(0xFF5B6B80))`.

</details>

---

<details>
<summary>File map</summary>

- `android/app/src/main/java/ke/co/baharilink/baharimrv/di/AppModule.kt` — added HTTP/1.1 protocol constraint for ngrok compatibility
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/AppRoot.kt` — replaced Material icons with wireframe SVGs, restyled navigation bar to 54dp height with 8.5sp labels and custom colors
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/BahariComponents.kt` — new 408-line component library implementing wireframe card, button, chip, icon background, gradient card, status pill, and bottom nav variants
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt` — updated `StatusChip` with wireframe colors and borders, updated `LedgerCard` to 12dp radius and 7dp gap
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/credits/CreditsScreen.kt` — adjusted padding (13dp/12dp), gap (14dp), card padding (10dp), price color (#059669), button radius (10dp), batch ID to grey pill, uppercase price label
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt` — added `ExtendedColorScheme` with cyan, violet, slate, navy, gradients; updated color palette (background #E9EEF4, surface #FFFFFF, containers #E9EEF4, outline #D8E0EA); added gradient definitions
- `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Type.kt` — remapped all typography styles to wireframe sizes (19sp display, 13.5sp app bar, 13sp card title, 11sp button, 10sp body, 9.5sp micro, 8.5sp nav)
- `android/app/src/main/res/drawable/*.xml` — added 28 wireframe icons (ic_home, ic_sat, ic_co2, ic_wallet, ic_user, ic_fish, ic_leaf, ic_boat, ic_book, ic_chart, ic_check, ic_bell, ic_search, ic_pin, ic_plus, ic_qr, ic_store, etc.)

Full diff: `git diff 4375a60..HEAD -- android/`

</details>
