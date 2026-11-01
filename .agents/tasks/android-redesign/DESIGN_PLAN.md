# BahariLink Android Redesign: Complete Design System & Implementation Plan

**Generated:** 2025-01-26  
**Source:** `/Users/Komarov/Documents/BahariMRV/baharilink-wireframe/index.html`  
**Target:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/`  
**Scope:** LARGE - Complete visual redesign with exact wireframe match

---

## Executive Summary

This plan extracts the complete design system from the BahariLink wireframe and maps it to Android Compose implementation. The wireframe is a comprehensive multi-module Blue Economy platform (Fisheries, Tourism, Conservation) with a sophisticated design system using gradients, custom icons, and precise spacing.

**Key Numbers:**
- **40+ SVG icons** to convert to Android vector drawables
- **15 color variables** + gradient definitions
- **12+ typography sizes** with negative letter spacing
- **8 border radius values** (8px to 44px for phone shell)
- **20+ Compose files** to modify/create
- **3 main modules** + shared identity system

---

## 1. COMPLETE DESIGN SYSTEM EXTRACTION

### 1.1 Color System (CSS Variables)

#### Base Colors
```css
--bg: #e9eef4           /* Background - light blue-grey */
--panel: #ffffff        /* Panel/card background - white */
--ink: #152238          /* Primary text - dark navy */
--mut: #5b6b80          /* Muted text - grey-blue */
--line: #d8e0ea         /* Border lines - light grey */
--wire: #b9c6d6         /* Wireframe dashed borders */
--sk: #dfe7f0           /* Skeleton loader - light */
--skd: #c7d2e0          /* Skeleton loader - darker */
--paper: #f7fafc        /* Paper background - off-white */
```

#### Accent Colors
```css
--cyan: #0e7490         /* Cyan - primary brand */
--blue: #2563eb         /* Blue - fisheries module */
--amber: #d97706        /* Amber - tourism module */
--green: #059669        /* Green - conservation module */
--violet: #7c3aed       /* Violet - cross-module synergies */
--slate: #475569        /* Slate - neutral badges */
--red: #dc2626          /* Red - errors/warnings */
```

#### Semantic Gradients
```css
/* Hero gradient (cyan-to-teal) */
background: linear-gradient(135deg, #082f3f 0%, #0e7490 55%, #155e75 100%);

/* Blue ID card gradient (navy-to-cyan) */
background: linear-gradient(135deg, #12263b, #0e7490);

/* Wallet card gradient (navy-to-blue) */
background: linear-gradient(135deg, #12263b, #1d4ed8);

/* Splash screen gradient (cyan-to-navy) */
background: linear-gradient(170deg, #0e7490, #082f3f);
```

#### Additional UI Colors
```css
/* Hero/splash accents */
--hero-text: #eaf6fa
--hero-accent: #7de3f5
--hero-kicker: #8fd6e8
--hero-sub: #c9e7f0
--hero-pill-border: rgba(255,255,255,.25)
--hero-legend-bg: rgba(8,47,63,.5)

/* Navy surfaces */
--navy: #0b1c2c         /* Tab bar background */
--navy-card: #12263b    /* Dark card background */
--navy-light: #1f2b3c   /* Pill offline background */
--navy-border: #24344b  /* Phone bezel shadow */

/* Status colors */
--offline-bg: #1f2b3c
--offline-text: #ffd166
--sync-bg: #e7f8f1
--sync-text: var(--green)
```

### 1.2 Typography System

#### Font Stack
```css
font-family: "Segoe UI", Inter, -apple-system, BlinkMacSystemFont, Roboto, Helvetica, Arial, sans-serif;
```
**Android Mapping:** Use `FontFamily.Default` (system font) as current implementation. Wireframe uses system fonts.

#### Font Sizes
```css
/* Display sizes */
34px    /* Hero h1 - letterSpacing: -0.5px */
24px    /* Section h2 - letterSpacing: -0.4px */
19px    /* Big numbers (.big) - letterSpacing: -0.3px, weight: 800 */

/* Title sizes */
15px    /* Hero subtitle */
13.5px  /* App bar title (.papp .ttl) - weight: 700 */
13px    /* Section title h4 - weight: 700 */
12.5px  /* Card title - weight: 700 */
12px    /* Medium body text - weight: 700 for labels */

/* Body sizes */
14px    /* Base body (default) - lineHeight: 1.5 */
11.5px  /* Caption paragraph */
11px    /* Small body (.notes, inputs, .btn) - weight: 700 for buttons */
10px    /* Smallest text (.papp .sub, .xs) - weight: 800 for badges */
9.5px   /* Micro labels (.ttl-xs) - weight: 800, letterSpacing: 0.08em */
9px     /* Pill text - weight: 700 */
8.5px   /* Bottom nav text (.nv) - weight: 600 */
8px     /* Splitbar text - weight: 700 */
```

#### Font Weights
```css
400     /* Normal - body text */
500     /* Medium - not explicitly used */
600     /* SemiBold - nav items, medium labels */
700     /* Bold - titles, buttons, strong emphasis */
800     /* ExtraBold - display numbers, badges, annotations */
```

#### Letter Spacing
```css
-0.5px  /* Hero h1 (34px) */
-0.4px  /* Section h2 (24px) */
-0.3px  /* Big numbers (19px) */
0.22em  /* Kicker text (11px uppercase) */
0.08em  /* Micro labels (.ttl-xs) */
0.05em  /* Small badges */
```

### 1.3 Spacing System

#### Padding & Gaps
```css
/* Component internal padding */
4px     /* Chip vertical, small gap */
5px     /* Badge padding */
6px     /* Input/button small padding */
7px-8px /* Card internal gaps */
9px     /* Medium card padding, button padding */
10px    /* Standard gap, card padding */
11px-12px /* Standard card padding */
13px    /* Tab padding, input padding */
14px    /* Board gap, legend padding */
16px    /* Large card padding, hero meta gap */
18px    /* Hero legend padding, tab container */

/* Layout spacing */
9px     /* Phone body gap */
12-13px /* Phone body padding */
26px    /* Section bottom margin */
30px    /* Wrap padding (desktop) */
34px    /* Hero top padding */
36px    /* Section top padding */
70px    /* Section bottom padding */

/* Phone UI */
54px    /* Bottom nav height (.pnav) */
30px    /* Status bar height (.pstat) */
22px    /* Notch height */
```

#### Border Radius
```css
4px     /* Tiny - legend swatch */
5px     /* Skeleton loader */
6px     /* Small badge, hash display */
7px     /* Splitbar */
8px     /* Small elements, num badge */
10px    /* Medium - inputs, buttons, cards */
11px    /* Flow boxes */
12px    /* Cards (.card), toast */
13px    /* Large card (KPI) */
14px    /* Hero legend, flows, large cards */
18px    /* Large shape */
20px    /* Icon large background */
34px    /* Phone screen inner radius */
44px    /* Phone outer shell, button pill */
999px   /* Fully rounded (pills, chips, switches) */
```

### 1.4 Component Specifications

#### Cards (.card)
```css
border: 1px solid var(--line)
border-radius: 12px
padding: 9px 10px
background: #fff
display: flex
flex-direction: column
gap: 7px

/* Highlighted variant (.card.hl) */
border-color: var(--blue)
box-shadow: 0 0 0 1px var(--blue) inset
```

#### Buttons (.btn)
```css
/* Base button */
border-radius: 10px
padding: 9px 10px
font-size: 11px
font-weight: 700
border: 1px solid var(--line)
background: #fff
color: var(--ink)
gap: 7px

/* Primary (.btn.p) */
background: var(--blue)
border-color: var(--blue)
color: #fff

/* Green (.btn.g) */
background: var(--green)
border-color: var(--green)
color: #fff

/* Amber (.btn.a) */
background: var(--amber)
border-color: var(--amber)
color: #fff

/* Violet (.btn.v) */
background: var(--violet)
border-color: var(--violet)
color: #fff

/* Ghost (.btn.ghost) */
color: var(--mut)

/* Small (.btn.sm) */
padding: 6px 8px
font-size: 10px
border-radius: 8px
```

#### Chips (.chip)
```css
/* Base chip */
border: 1px solid var(--line)
border-radius: 999px
padding: 4px 9px
font-size: 9.5px
font-weight: 600
color: var(--mut)
background: #fff
gap: 5px

/* Active (.chip.on) */
background: #eaf1ff
border-color: #bfd4ff
color: var(--blue)

/* Green (.chip.green) */
background: #e7f8f1
border-color: #b7e8d4
color: var(--green)

/* Amber (.chip.amber) */
background: #fdf3e3
border-color: #f2ddb3
color: var(--amber)
```

#### Pills (status indicators)
```css
/* Hero pill (.hero .pill) */
background: rgba(255,255,255,.1)
border: 1px solid rgba(255,255,255,.25)
color: #dff4fa
border-radius: 999px
padding: 6px 13px
font-size: 12px
gap: 7px

/* Offline pill (.pill-off) */
background: #1f2b3c
color: #ffd166
border-radius: 999px
padding: 3px 8px
font-size: 9px
font-weight: 700

/* Sync pill (.pill-sync) */
background: #e7f8f1
color: var(--green)
border-radius: 999px
padding: 3px 8px
font-size: 9px
font-weight: 700
```

#### Status Badges (.sid, .num, .an)
```css
/* Section number (.num) */
font-size: 12px
font-weight: 800
color: #fff
background: var(--cyan)
border-radius: 8px
padding: 5px 10px
letter-spacing: 0.08em

/* Screen ID (.sid) */
font-size: 10px
font-weight: 800
color: #fff
background: var(--slate)
border-radius: 6px
padding: 2px 7px
letter-spacing: 0.05em

/* Annotation number (.an) */
width: 16px
height: 16px
border-radius: 50%
background: #f59e0b
color: #231a04
font-size: 10px
font-weight: 800
display: inline-flex
align-items: center
justify-content: center
```

#### Icon Backgrounds (.icn)
```css
/* Base icon background */
width: 34px
height: 34px
border-radius: 10px
background: #e8eef6
color: var(--slate)

/* Blue variant (.icn.blue) */
background: #dbeafe
color: var(--blue)

/* Green variant (.icn.green) */
background: #d1fae5
color: var(--green)

/* Amber variant (.icn.amber) */
background: #fef3c7
color: var(--amber)

/* Violet variant (.icn.violet) */
background: #ede9fe
color: var(--violet)

/* Cyan variant (.icn.cyan) */
background: #cffafe
color: var(--cyan)
```

#### Input Fields (.inp)
```css
border: 1px solid var(--line)
border-radius: 10px
padding: 8px 10px
font-size: 11px
color: var(--mut)
background: #fff
gap: 7px
```

#### Bottom Navigation (.pnav)
```css
height: 54px
border-top: 1px solid var(--line)
background: #fff

/* Nav item (.nv) */
font-size: 8.5px
color: #9aa8ba
font-weight: 600
gap: 3px

/* Active nav item (.nv.on) */
color: var(--blue)

/* Color variants for different modules */
.pnav.green .nv.on { color: var(--green) }
.pnav.amber .nv.on { color: var(--amber) }
.pnav.violet .nv.on { color: var(--violet) }
```

#### Phone Shell (.ph)
```css
width: 296px
height: 608px
background: #0a1420
border-radius: 44px
padding: 11px
box-shadow: 0 18px 40px rgba(15,35,60,.28), inset 0 0 0 2px #24344b

/* Phone screen (.pscr) */
background: #fff
border-radius: 34px
height: 100%

/* Notch */
width: 110px
height: 22px
background: #0a1420
border-radius: 0 0 14px 14px
```

#### Toast Notification (.toast)
```css
/* Dark toast */
background: #12263b
color: #dff2fb
border-radius: 12px
padding: 9px 11px
font-size: 10px
gap: 8px

/* Success toast (green variant) */
background: #e7f8f1
color: #065f46
```

#### Dashed Placeholder (.dash)
```css
border: 1.5px dashed var(--wire)
border-radius: 12px
background: repeating-linear-gradient(45deg, #f7fafc, #f7fafc 9px, #edf3f8 9px, #edf3f8 18px)
color: #8ca0b6
font-size: 10px
font-weight: 600
gap: 8px
```

#### Switch (.sw)
```css
/* Active switch */
width: 34px
height: 20px
border-radius: 999px
background: var(--green)
position: relative

/* Switch knob (::after) */
width: 14px
height: 14px
border-radius: 50%
background: #fff
right: 3px
top: 3px

/* Inactive switch (.sw.off) */
background: #c9d4e2
/* knob left: 3px */
```

#### Progress Bar (.bar)
```css
height: 6px
border-radius: 4px
background: var(--sk)

/* Fill (bar i) */
height: 100%
border-radius: 4px
background: var(--green)
```

#### Hash Display (.hash)
```css
font-family: Consolas, monospace
font-size: 8.5px
color: var(--mut)
background: #f1f5f9
border-radius: 6px
padding: 3px 6px
overflow: hidden
text-overflow: ellipsis
white-space: nowrap
```

### 1.5 SVG Icon Library (Complete List)

All icons use these properties:
```css
stroke: currentColor
fill: none
stroke-width: 1.8
stroke-linecap: round
stroke-linejoin: round
```

#### Icon Sizes
```css
.ic (base): 18×18px
.ic.s (small): 14×14px
.ic.l (large): 22×22px
.ic.xl (extra large): 30×30px
```

#### Complete Icon Inventory (40 icons)

1. **i-home** - Home (house with roof)
2. **i-book** - Book/logbook (open book with lines)
3. **i-store** - Store/marketplace (shop front)
4. **i-wallet** - Wallet (wallet with circle accent)
5. **i-user** - User (single person circle)
6. **i-users** - Users (multiple people)
7. **i-fish** - Fish (fish outline with eye)
8. **i-leaf** - Leaf/conservation (single leaf)
9. **i-cal** - Calendar (calendar grid)
10. **i-qr** - QR code (QR pattern)
11. **i-sat** - Satellite (satellite with diamond center)
12. **i-shield** - Shield/verification (shield with checkmark)
13. **i-chart** - Chart/analytics (bar chart)
14. **i-cam** - Camera (camera outline)
15. **i-mic** - Microphone (microphone with stand)
16. **i-bell** - Bell/notifications (bell outline)
17. **i-search** - Search (magnifying glass)
18. **i-pin** - Pin/location (map pin with circle)
19. **i-globe** - Globe (globe with latitude lines)
20. **i-check** - Check (checkmark)
21. **i-clock** - Clock (clock with hands)
22. **i-plus** - Plus (plus sign)
23. **i-star** - Star (star outline)
24. **i-anchor** - Anchor (anchor with ring)
25. **i-waves** - Waves (wavy lines)
26. **i-doc** - Document (document with lines)
27. **i-scan** - Scan (scan frame corners)
28. **i-sync** - Sync (circular arrows)
29. **i-send** - Send (paper plane)
30. **i-boat** - Boat (sailboat)
31. **i-card** - Card/payment (credit card)
32. **i-db** - Database (cylinder stack)
33. **i-link** - Link (chain links)
34. **i-tts** - Text-to-speech (speaker with waves)
35. **i-arr** - Arrow (right arrow)
36. **i-bat** - Battery (battery indicator)
37. **i-sig** - Signal (signal bars)
38. **i-co2** - Cloud/CO2 (cloud outline)
39. **i-gift** - Gift (gift box with bow)

---

## 2. ICON CONVERSION PLAN

### 2.1 SVG to Android Vector Drawable Conversion

**Target Directory:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/res/drawable/`

#### Conversion Template
```xml
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="24"
    android:viewportHeight="24">
    <path
        android:pathData="[SVG path data]"
        android:strokeColor="@android:color/black"
        android:strokeWidth="1.8"
        android:strokeLineCap="round"
        android:strokeLineJoin="round"
        android:fillColor="@android:color/transparent"/>
</vector>
```

#### Priority Icons (Must Convert First - 15 icons)

These appear in the core navigation and most common screens:

1. **ic_home.xml** - Bottom nav home
2. **ic_book.xml** - Bottom nav logbook
3. **ic_store.xml** - Bottom nav market
4. **ic_wallet.xml** - Bottom nav wallet
5. **ic_user.xml** - Bottom nav profile
6. **ic_fish.xml** - Fisheries module
7. **ic_leaf.xml** - Conservation module
8. **ic_boat.xml** - Tourism module
9. **ic_check.xml** - Verification checkmarks
10. **ic_qr.xml** - Traceability QR codes
11. **ic_pin.xml** - Location markers
12. **ic_bell.xml** - Notifications
13. **ic_search.xml** - Search functionality
14. **ic_plus.xml** - Add actions
15. **ic_arr.xml** - Navigation arrows

#### Secondary Icons (Batch 2 - 15 icons)

16. **ic_shield.xml** - Verification/credentials
17. **ic_cam.xml** - Photo capture
18. **ic_mic.xml** - Voice input
19. **ic_clock.xml** - Time/history
20. **ic_sync.xml** - Offline sync
21. **ic_sat.xml** - Satellite monitoring
22. **ic_chart.xml** - Analytics/metrics
23. **ic_waves.xml** - Ocean/marine
24. **ic_anchor.xml** - Marine/anchor
25. **ic_doc.xml** - Documents
26. **ic_scan.xml** - Scanning
27. **ic_send.xml** - Send/share
28. **ic_globe.xml** - Global/network
29. **ic_link.xml** - Blockchain/connection
30. **ic_card.xml** - Payment/M-Pesa

#### Tertiary Icons (Batch 3 - 10 icons)

31. **ic_users.xml** - Community/groups
32. **ic_cal.xml** - Calendar/scheduling
33. **ic_star.xml** - Ratings/favorites
34. **ic_db.xml** - Database/storage
35. **ic_tts.xml** - Voice guidance
36. **ic_bat.xml** - Battery status
37. **ic_sig.xml** - Signal strength
38. **ic_co2.xml** - Carbon/emissions
39. **ic_gift.xml** - Rewards/benefits
40. **ic_boat.xml** - Boat/vessel

### 2.2 Icon Usage in Compose

**Create Icon Helper:**
`/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Icons.kt`

```kotlin
package ke.co.baharilink.baharimrv.ui.components

import androidx.annotation.DrawableRes
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import ke.co.baharilink.baharimrv.R

enum class BahariIcon(@DrawableRes val resourceId: Int) {
    HOME(R.drawable.ic_home),
    BOOK(R.drawable.ic_book),
    STORE(R.drawable.ic_store),
    WALLET(R.drawable.ic_wallet),
    USER(R.drawable.ic_user),
    FISH(R.drawable.ic_fish),
    LEAF(R.drawable.ic_leaf),
    BOAT(R.drawable.ic_boat),
    CHECK(R.drawable.ic_check),
    QR(R.drawable.ic_qr),
    PIN(R.drawable.ic_pin),
    BELL(R.drawable.ic_bell),
    SEARCH(R.drawable.ic_search),
    PLUS(R.drawable.ic_plus),
    ARROW(R.drawable.ic_arr),
    // ... rest of icons
}

@Composable
fun BahariIcon(
    icon: BahariIcon,
    contentDescription: String?,
    modifier: Modifier = Modifier
) {
    Icon(
        painter = painterResource(icon.resourceId),
        contentDescription = contentDescription,
        modifier = modifier
    )
}
```

---

## 3. ANDROID COMPOSE IMPLEMENTATION PLAN

### 3.1 Theme System Update

**File:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Theme.kt`

#### Current vs Wireframe Color Mapping

| Wireframe CSS | Current Android | Action |
|---------------|----------------|--------|
| `--bg: #e9eef4` | `background: #fdfefe` | **UPDATE** to `#E9EEF4` |
| `--panel: #ffffff` | `surface: #ffffff` | ✓ Match |
| `--ink: #152238` | `onSurface: #152238` | ✓ Match |
| `--mut: #5b6b80` | `onSurfaceVariant: #5b6b80` | ✓ Match |
| `--line: #d8e0ea` | `outline: #d8e0ea` | ✓ Match |
| `--cyan: #0e7490` | Missing | **ADD** as custom color |
| `--blue: #2563eb` | `secondary: #2563eb` | ✓ Match |
| `--amber: #d97706` | `tertiary: #d97706` | ✓ Match |
| `--green: #059669` | `primary: #059669` | ✓ Match |
| `--violet: #7c3aed` | Missing | **ADD** as custom color |
| `--red: #dc2626` | `error: #ba1a1a` | **UPDATE** to `#DC2626` |

#### Add Extended Color Scheme

```kotlin
@Immutable
data class ExtendedColorScheme(
    val cyan: Color,
    val onCyan: Color,
    val cyanContainer: Color,
    val onCyanContainer: Color,
    val violet: Color,
    val onViolet: Color,
    val violetContainer: Color,
    val onVioletContainer: Color,
    val slate: Color,
    val onSlate: Color,
    val navy: Color,
    val navyLight: Color,
    val offlineYellow: Color,
    val emeraldGreen: Color,
)

val LightExtendedColors = ExtendedColorScheme(
    cyan = Color(0xFF0E7490),
    onCyan = Color.White,
    cyanContainer = Color(0xFFCFFAFE),
    onCyanContainer = Color(0xFF0C4A5E),
    violet = Color(0xFF7C3AED),
    onViolet = Color.White,
    violetContainer = Color(0xFFEDE9FE),
    onVioletContainer = Color(0xFF4C1D95),
    slate = Color(0xFF475569),
    onSlate = Color.White,
    navy = Color(0xFF0B1C2C),
    navyLight = Color(0xFF12263B),
    offlineYellow = Color(0xFFFFD166),
    emeraldGreen = Color(0xFF079B76),
)
```

### 3.2 Typography System Update

**File:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/theme/Type.kt`

#### Current Issues
- Missing negative letter spacing for display text
- Missing 19sp "big" size
- Missing micro label styles (9.5sp, 8.5sp)
- Missing annotation badge style (10sp extra bold)

#### Wireframe-Aligned Typography

```kotlin
val BahariTypography = Typography(
    // Display - for hero headers and big numbers
    displayLarge = TextStyle(  // Hero H1 - 34sp
        fontWeight = FontWeight.Bold,
        fontSize = 34.sp,
        lineHeight = 40.sp,
        letterSpacing = (-0.5).sp,
    ),
    displayMedium = TextStyle(  // Section H2 - 24sp
        fontWeight = FontWeight.Bold,
        fontSize = 24.sp,
        lineHeight = 30.sp,
        letterSpacing = (-0.4).sp,
    ),
    displaySmall = TextStyle(  // Big numbers - 19sp
        fontWeight = FontWeight.ExtraBold,
        fontSize = 19.sp,
        lineHeight = 24.sp,
        letterSpacing = (-0.3).sp,
    ),
    
    // Headlines - for card titles
    headlineMedium = TextStyle(  // 13.5sp app bar title
        fontWeight = FontWeight.Bold,
        fontSize = 13.5.sp,
        lineHeight = 18.sp,
        letterSpacing = 0.sp,
    ),
    headlineSmall = TextStyle(  // 13sp section title
        fontWeight = FontWeight.Bold,
        fontSize = 13.sp,
        lineHeight = 17.sp,
        letterSpacing = 0.sp,
    ),
    
    // Titles
    titleLarge = TextStyle(  // 12.5sp card title
        fontWeight = FontWeight.Bold,
        fontSize = 12.5.sp,
        lineHeight = 17.sp,
        letterSpacing = 0.sp,
    ),
    titleMedium = TextStyle(  // 12sp medium labels
        fontWeight = FontWeight.Bold,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.sp,
    ),
    titleSmall = TextStyle(  // 11sp small labels
        fontWeight = FontWeight.Bold,
        fontSize = 11.sp,
        lineHeight = 15.sp,
        letterSpacing = 0.sp,
    ),
    
    // Body - standard text
    bodyLarge = TextStyle(  // 14sp base body
        fontWeight = FontWeight.Normal,
        fontSize = 14.sp,
        lineHeight = 21.sp,
        letterSpacing = 0.sp,
    ),
    bodyMedium = TextStyle(  // 11.5sp caption
        fontWeight = FontWeight.Normal,
        fontSize = 11.5.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.sp,
    ),
    bodySmall = TextStyle(  // 11sp notes
        fontWeight = FontWeight.Normal,
        fontSize = 11.sp,
        lineHeight = 15.5.sp,
        letterSpacing = 0.sp,
    ),
    
    // Labels - for UI chrome
    labelLarge = TextStyle(  // 11sp button text
        fontWeight = FontWeight.Bold,
        fontSize = 11.sp,
        lineHeight = 15.sp,
        letterSpacing = 0.sp,
    ),
    labelMedium = TextStyle(  // 10sp sub-text
        fontWeight = FontWeight.ExtraBold,
        fontSize = 10.sp,
        lineHeight = 14.sp,
        letterSpacing = 0.sp,
    ),
    labelSmall = TextStyle(  // 9.5sp micro labels (ttl-xs)
        fontWeight = FontWeight.ExtraBold,
        fontSize = 9.5.sp,
        lineHeight = 13.sp,
        letterSpacing = 0.08.em,
    ),
)
```

**Add Custom Typography Extensions:**

```kotlin
// Additional wireframe sizes not in Material3 scale
val Typography.heroSubtitle: TextStyle
    @Composable get() = TextStyle(
        fontWeight = FontWeight.Normal,
        fontSize = 15.sp,
        lineHeight = 22.sp,
    )

val Typography.pillText: TextStyle
    @Composable get() = TextStyle(
        fontWeight = FontWeight.Bold,
        fontSize = 9.sp,
        lineHeight = 12.sp,
    )

val Typography.navText: TextStyle
    @Composable get() = TextStyle(
        fontWeight = FontWeight.SemiBold,
        fontSize = 8.5.sp,
        lineHeight = 12.sp,
    )

val Typography.annotationBadge: TextStyle
    @Composable get() = TextStyle(
        fontWeight = FontWeight.ExtraBold,
        fontSize = 10.sp,
        lineHeight = 10.sp,
    )
```

### 3.3 Shape System Update

**Current:**
```kotlin
shapes = Shapes(
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(18.dp),
)
```

**Wireframe-Aligned:**
```kotlin
shapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp),   // badges, hash
    small = RoundedCornerShape(8.dp),        // small buttons, badges
    medium = RoundedCornerShape(10.dp),      // buttons, inputs, icons
    large = RoundedCornerShape(12.dp),       // cards, main UI
    extraLarge = RoundedCornerShape(14.dp),  // large cards, legends
)

// Custom shapes for specific components
val BahariShapes = Shapes(
    // ... Material3 shapes above ...
)

// Extension properties
val Shapes.pill: RoundedCornerShape
    get() = RoundedCornerShape(999.dp)

val Shapes.phoneScreen: RoundedCornerShape
    get() = RoundedCornerShape(34.dp)

val Shapes.phoneShell: RoundedCornerShape
    get() = RoundedCornerShape(44.dp)
```

### 3.4 Component Library Creation

**New File:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/BahariComponents.kt`

#### Component Inventory

1. **BahariCard** - Standard card with border (replaces basic Card usage)
2. **BahariButton** - Primary, secondary, ghost variants
3. **BahariChip** - Status chips with color variants
4. **BahariPill** - Status pills (offline, sync)
5. **BahariInput** - Standard text input field
6. **BahariIconBackground** - Colored icon containers
7. **BahariBottomNav** - Bottom navigation with module colors
8. **BahariToast** - Toast notification (dark/success variants)
9. **BahariDashedBorder** - Dashed placeholder modifier
10. **BahariSwitch** - Custom switch component
11. **BahariProgressBar** - Thin progress indicator
12. **BahariHash** - Monospace hash display
13. **BahariAnnotationBadge** - Numbered annotation circles
14. **BahariStatusBadge** - Section/screen ID badges
15. **BahariGradientCard** - Gradient card variants

#### Example: BahariCard Component

```kotlin
@Composable
fun BahariCard(
    modifier: Modifier = Modifier,
    highlighted: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val borderColor = if (highlighted) {
        MaterialTheme.colorScheme.secondary
    } else {
        MaterialTheme.colorScheme.outline
    }
    
    Card(
        modifier = modifier.fillMaxWidth().let {
            if (onClick != null) it.clickable(onClick = onClick) else it
        },
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        border = BorderStroke(
            width = 1.dp,
            color = borderColor
        )
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 9.dp),
            verticalArrangement = Arrangement.spacedBy(7.dp),
            content = content
        )
    }
}
```

#### Example: BahariButton Component

```kotlin
enum class BahariButtonVariant {
    PRIMARY, GREEN, AMBER, VIOLET, GHOST, OUTLINED
}

@Composable
fun BahariButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: BahariButtonVariant = BahariButtonVariant.PRIMARY,
    small: Boolean = false,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit
) {
    val colors = MaterialTheme.colorScheme
    val (containerColor, contentColor, borderColor) = when (variant) {
        BahariButtonVariant.PRIMARY -> Triple(
            colors.secondary, colors.onSecondary, colors.secondary
        )
        BahariButtonVariant.GREEN -> Triple(
            colors.primary, colors.onPrimary, colors.primary
        )
        BahariButtonVariant.AMBER -> Triple(
            colors.tertiary, colors.onTertiary, colors.tertiary
        )
        BahariButtonVariant.VIOLET -> Triple(
            Color(0xFF7C3AED), Color.White, Color(0xFF7C3AED)
        )
        BahariButtonVariant.GHOST -> Triple(
            Color.Transparent, colors.onSurfaceVariant, Color.Transparent
        )
        BahariButtonVariant.OUTLINED -> Triple(
            colors.surface, colors.onSurface, colors.outline
        )
    }
    
    Button(
        onClick = onClick,
        modifier = modifier.height(if (small) 36.dp else 48.dp),
        enabled = enabled,
        shape = RoundedCornerShape(if (small) 8.dp else 10.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = colors.surfaceVariant,
            disabledContentColor = colors.onSurfaceVariant
        ),
        border = if (variant == BahariButtonVariant.OUTLINED) {
            BorderStroke(1.dp, borderColor)
        } else null,
        contentPadding = PaddingValues(
            horizontal = if (small) 8.dp else 10.dp,
            vertical = if (small) 6.dp else 9.dp
        )
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(7.dp),
            verticalAlignment = Alignment.CenterVertically,
            content = content
        )
    }
}
```

#### Example: BahariChip Component

```kotlin
@Composable
fun BahariChip(
    text: String,
    modifier: Modifier = Modifier,
    selected: Boolean = false,
    color: ChipColor = ChipColor.NEUTRAL,
    leadingIcon: @Composable (() -> Unit)? = null,
    onClick: (() -> Unit)? = null
) {
    val colors = MaterialTheme.colorScheme
    val (background, border, textColor) = when {
        selected && color == ChipColor.NEUTRAL -> Triple(
            Color(0xFFEAF1FF), Color(0xFFBFD4FF), colors.secondary
        )
        selected && color == ChipColor.GREEN -> Triple(
            Color(0xFFE7F8F1), Color(0xFFB7E8D4), colors.primary
        )
        selected && color == ChipColor.AMBER -> Triple(
            Color(0xFFFDF3E3), Color(0xFFF2DDB3), colors.tertiary
        )
        else -> Triple(
            colors.surface, colors.outline, colors.onSurfaceVariant
        )
    }
    
    Surface(
        modifier = modifier.let {
            if (onClick != null) it.clickable(onClick = onClick) else it
        },
        color = background,
        contentColor = textColor,
        shape = CircleShape,
        border = BorderStroke(1.dp, border)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 9.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(5.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            leadingIcon?.invoke()
            Text(
                text = text,
                style = MaterialTheme.typography.bodySmall.copy(
                    fontSize = 9.5.sp,
                    fontWeight = FontWeight.SemiBold
                )
            )
        }
    }
}

enum class ChipColor { NEUTRAL, GREEN, AMBER }
```

### 3.5 Bottom Navigation Redesign

**File:** `/Users/Komarov/Documents/BahariMRV/android/app/src/main/java/ke/co/baharilink/baharimrv/ui/AppRoot.kt` (or navigation component)

#### Wireframe Bottom Nav Specs
- Height: 54dp (vs current Material3 80dp)
- Font size: 8.5sp (vs current 12sp)
- Icon size: 18dp standard (vs current 24dp)
- Active color: Module-specific (blue/green/amber/violet)
- Inactive color: #9AA8BA

```kotlin
@Composable
fun BahariBottomNavigation(
    selectedRoute: String,
    onNavigate: (String) -> Unit,
    moduleColor: Color = MaterialTheme.colorScheme.secondary
) {
    NavigationBar(
        modifier = Modifier.height(54.dp),
        containerColor = MaterialTheme.colorScheme.surface,
        tonalElevation = 0.dp
    ) {
        BahariNavItem(
            selected = selectedRoute == "home",
            onClick = { onNavigate("home") },
            icon = BahariIcon.HOME,
            label = "Home",
            activeColor = moduleColor
        )
        BahariNavItem(
            selected = selectedRoute == "projects",
            onClick = { onNavigate("projects") },
            icon = BahariIcon.BOOK,
            label = "Projects",
            activeColor = moduleColor
        )
        // ... other nav items
    }
}

@Composable
fun RowScope.BahariNavItem(
    selected: Boolean,
    onClick: () -> Unit,
    icon: BahariIcon,
    label: String,
    activeColor: Color
) {
    NavigationBarItem(
        selected = selected,
        onClick = onClick,
        icon = {
            Icon(
                painter = painterResource(icon.resourceId),
                contentDescription = label,
                modifier = Modifier.size(18.dp)
            )
        },
        label = {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall.copy(
                    fontSize = 8.5.sp,
                    fontWeight = FontWeight.SemiBold
                )
            )
        },
        colors = NavigationBarItemDefaults.colors(
            selectedIconColor = activeColor,
            selectedTextColor = activeColor,
            unselectedIconColor = Color(0xFF9AA8BA),
            unselectedTextColor = Color(0xFF9AA8BA),
            indicatorColor = Color.Transparent
        )
    )
}
```

---

## 4. FILE-BY-FILE IMPLEMENTATION ORDER

### Phase 1: Foundation (Days 1-2)

#### 1.1 Theme Updates
- **Theme.kt** - Update color scheme with wireframe colors, add extended colors
- **Type.kt** - Update typography with wireframe font sizes and weights
- **Theme.kt** (shapes) - Update shape system with wireframe radii

#### 1.2 Icon Conversion (Priority Batch)
- Convert 15 priority icons (home, book, store, wallet, user, fish, leaf, boat, check, qr, pin, bell, search, plus, arr)
- Create **Icons.kt** helper file

### Phase 2: Component Library (Days 3-4)

#### 2.1 Core Components
- **BahariComponents.kt** - Create new component file with:
  - BahariCard
  - BahariButton (all variants)
  - BahariChip
  - BahariPill
  - BahariInput

#### 2.2 Additional Components
- **BahariComponents.kt** (continued):
  - BahariIconBackground
  - BahariToast
  - BahariHash
  - BahariAnnotationBadge
  - BahariStatusBadge

#### 2.3 Extended Components
- **BahariAdvancedComponents.kt**:
  - BahariGradientCard
  - BahariSwitch
  - BahariProgressBar
  - BahariDashedPlaceholder

### Phase 3: Navigation & Shell (Day 5)

#### 3.1 Bottom Navigation
- **AppRoot.kt** - Update bottom navigation with wireframe styling
- Create **BahariBottomNavigation.kt** if needed

#### 3.2 App Bar
- **Common.kt** - Update ContextHeader to match wireframe app bar specs

### Phase 4: Screen Redesigns (Days 6-10)

#### 4.1 Home Screen
- **HomeScreen.kt** - Replace components with Bahari components
  - Update cards to BahariCard
  - Update buttons to BahariButton
  - Update chips to BahariChip
  - Match spacing to wireframe (10dp gaps, 12dp padding)
  - Update wallet card with gradient

#### 4.2 Projects Screen
- **ProjectsScreen.kt** - Apply Bahari components
  - Update project row cards
  - Update status chips
  - Match card padding (12dp)

#### 4.3 Project Detail Screen
- **ProjectDetailScreen.kt** - Apply Bahari components
  - Update metric cards
  - Update action buttons
  - Update spacing

#### 4.4 Monitor Screen
- **MonitorScreen.kt** - Apply Bahari components
  - Update field visit cards
  - Update status indicators
  - Update forms with BahariInput

#### 4.5 Add Plot/Activity Screens
- **AddPlotScreen.kt** - Apply Bahari components
- **AddActivityScreen.kt** - Apply Bahari components
  - Update all input fields
  - Update buttons
  - Update spacing

#### 4.6 Verification Screen
- **VerificationScreen.kt** - Apply Bahari components
  - Update report cards
  - Update status chips
  - Add stage indicator circles

#### 4.7 Wallet Screen
- **WalletScreen.kt** - Major redesign
  - Implement gradient card
  - Update revenue stream rows
  - Match wireframe wallet styling exactly

#### 4.8 Credits Screen
- **CreditsScreen.kt** - Apply Bahari components
  - Update credit cards
  - Update retirement certificates (navy card)

#### 4.9 Profile Screen
- **ProfileScreen.kt** - Apply Bahari components
  - Update credential cards
  - Update user info display

#### 4.10 Auth Screen
- **AuthScreen.kt** - Major redesign
  - Implement gradient splash screen styling
  - Update login UI with wireframe colors

### Phase 5: Icon Completion (Day 11)

#### 5.1 Secondary Icons
- Convert remaining 25 icons (shield, cam, mic, clock, sync, sat, chart, waves, etc.)
- Update all screen icon usages

### Phase 6: Polish & Refinement (Day 12)

#### 6.1 Spacing Audit
- Review all screens for exact wireframe spacing
- Update padding values to match (9dp, 10dp, 12dp patterns)

#### 6.2 Color Audit
- Verify all colors match wireframe exactly
- Update any hardcoded colors

#### 6.3 Typography Audit
- Verify font sizes match wireframe
- Verify font weights match wireframe

---

## 5. TESTING STRATEGY

### 5.1 Visual Regression Testing

#### 5.1.1 Side-by-Side Comparison
- Open wireframe in browser
- Run app on emulator/device side-by-side
- Compare each screen systematically

#### 5.1.2 Screenshot Comparison
- Take screenshots of wireframe screens
- Take screenshots of app screens
- Overlay and compare in image editor

#### 5.1.3 Measurement Verification
- Use Android Studio Layout Inspector
- Verify:
  - Card padding (should be 12dp)
  - Button heights (should be 48dp standard, 36dp small)
  - Icon sizes (should be 18dp standard)
  - Bottom nav height (should be 54dp)
  - Text sizes (match sp values from wireframe)
  - Border radius values (8dp, 10dp, 12dp)

### 5.2 Color Verification

#### 5.2.1 Color Picker Test
- Use color picker tool on wireframe
- Compare to app hex values
- Verify Material3 color role mapping

#### 5.2.2 Key Colors to Verify
- Background: #E9EEF4
- Primary green: #059669
- Secondary blue: #2563EB
- Tertiary amber: #D97706
- Cyan brand: #0E7490
- Violet: #7C3AED
- Navy: #12263B
- Line/border: #D8E0EA

### 5.3 Typography Verification

#### 5.3.1 Text Size Audit
Verify these specific sizes:
- Hero title: 34sp
- Section title: 24sp
- Big numbers: 19sp
- Card titles: 12.5sp, 13sp
- Body text: 11sp, 11.5sp, 14sp
- Micro labels: 9.5sp
- Nav text: 8.5sp
- Pills: 9sp

#### 5.3.2 Font Weight Verification
- Verify ExtraBold (800) usage for badges
- Verify Bold (700) usage for titles
- Verify SemiBold (600) usage for labels

### 5.4 Spacing Verification

#### 5.4.1 Component Internal Spacing
- Card padding: 9-12dp (most common: 12dp)
- Button padding: 9-10dp horizontal
- Chip padding: 4dp vertical, 9dp horizontal
- Icon backgrounds: 34dp × 34dp
- Gap between card items: 7-8dp

#### 5.4.2 Layout Spacing
- Screen padding: 16dp
- Section gaps: 10dp (standard list spacing)
- Bottom nav height: 54dp
- App bar padding: match wireframe

### 5.5 Functionality Preservation

#### 5.5.1 Navigation Tests
- Bottom navigation switches screens
- Back navigation works
- Deep links work
- Module switching works

#### 5.5.2 Interaction Tests
- Buttons trigger actions
- Cards are clickable where expected
- Chips toggle selection
- Forms submit correctly
- Offline mode still works
- Sync still works

#### 5.5.3 Data Display Tests
- All current data still displays
- No data loss in redesign
- API responses still render
- Error states still show

### 5.6 Accessibility Testing

#### 5.6.1 Minimum Touch Targets
- Verify buttons meet 48dp minimum
- Verify nav items meet 48dp minimum
- Verify small chips remain tappable

#### 5.6.2 Content Descriptions
- All icons have contentDescription
- All images have descriptions
- All interactive elements labeled

#### 5.6.3 Text Contrast
- Verify all text meets WCAG AA (4.5:1)
- Verify muted text (#5B6B80) on white is readable
- Verify white text on colored backgrounds

---

## 6. DETAILED SCREEN-BY-SCREEN WIREFRAME MAPPING

### 6.1 Module 0: Identity & Onboarding Screens

#### S1: Splash & Language (Not in current app)
**Wireframe Features:**
- Gradient background: `linear-gradient(170deg, #0e7490, #082f3f)`
- Large icon: 64×64dp, rounded 20dp, semi-transparent white bg
- Title: 24sp, bold, white with cyan accent
- Language buttons: full-width, one primary (white), one ghost (transparent)
- Voice guidance row with speaker icon

**Implementation Plan:**
- Create new **SplashScreen.kt**
- Use Box with gradient background
- Implement language picker with BahariButton
- Add to onboarding flow if needed

#### S2: Role Selection (Not in current app)
**Wireframe Features:**
- Card grid with 4 role options
- Each card: icon background (34×34dp), title (12.5sp bold), subtitle (9.5sp muted)
- One card highlighted with blue border
- Toast notification at bottom

**Implementation Plan:**
- Create **RoleSelectionScreen.kt** if onboarding is implemented
- Use BahariCard with highlighted variant
- BahariIconBackground for role icons

#### S3: Phone + M-Pesa Verification (Related to Auth)
**Wireframe Features:**
- Input fields with country code prefix
- OTP code grid (4 boxes)
- Switch toggle for M-Pesa linking
- Optional co-op code input
- USSD fallback button (ghost, small)

**Current:** AuthScreen.kt exists but different flow
**Action:** Update AuthScreen to match wireframe if login flow changes

#### S4: Blue ID Card (Not in current app)
**Wireframe Features:**
- Gradient card (navy to cyan)
- QR code display
- Verifiable credentials list with shield icons
- "Recovery guardians" toast

**Implementation Plan:**
- Create **ProfileDetailScreen.kt** or integrate into ProfileScreen
- Use BahariGradientCard
- Display credentials as cards with icons

### 6.2 Module 1: Fisheries Screens (Reference for Conservation App)

The wireframe shows Fisheries module screens (F1-F5). Current app is Conservation-focused, so mapping is conceptual:

#### F1: Fisher Home Dashboard → **HomeScreen.kt**
**Wireframe Features:**
- Offline pill in app bar
- Weather/tide card (highlighted blue)
- Price grid (3 columns)
- Catch summary card
- Two-button action row (split 1:1)
- Bottom nav with 5 items

**Current Implementation:**
- Has dashboard with project selector
- Has metrics in 2-column grid
- Has alert cards
- Needs: Module-colored bottom nav, offline indicator

**Changes:**
- Add offline pill to ContextHeader
- Update metric grid to match wireframe 2-column layout
- Update button row to full-width split layout
- Reduce bottom nav height to 54dp

#### F5: Wallet & Income → **WalletScreen.kt**
**Wireframe Features:**
- Gradient balance card (navy to blue)
- Balance: large 19sp number
- Cash out / History button row inside card
- Earnings card with bar chart visualization
- Income credential card (green container)
- Co-benefit contributor card (violet icon)

**Current Implementation:**
- Has wallet card with balances
- Has revenue stream rows
- Missing: gradient background, bar chart, credential cards

**Changes:**
- Replace wallet card with BahariGradientCard
- Add gradient background (Color(0xFF12263B) to Color(0xFF1D4ED8))
- Implement bar chart visualization for 6-month earnings
- Add credential display cards

### 6.3 Current App Screens → Wireframe Styling

#### **HomeScreen.kt** - Dashboard
**Current Issues:**
- Cards use default Material3 styling
- Buttons use default heights (likely 40dp)
- Spacing is generic (16dp everywhere)
- No module color in navigation

**Wireframe-Aligned Changes:**
- Replace `Card` with `BahariCard(contentPadding = 12.dp)`
- Replace `Button` with `BahariButton(variant = PRIMARY)`
- Update gaps to 10dp: `Arrangement.spacedBy(10.dp)`
- Update map dashed border to match wireframe style
- Update metric cards to compact style (padding: 12dp)
- Update wallet card to gradient style

#### **ProjectsScreen.kt** - Project List
**Current:**
- Uses Card for project rows
- Uses StatusChip (already close to wireframe)
- Standard padding

**Changes:**
- Replace Card with BahariCard
- Verify StatusChip styling matches wireframe pill style
- Update padding to 12dp
- Update gaps to 10dp

#### **ProjectDetailScreen.kt** - Project Details
**Current:**
- Uses standard cards
- Uses buttons

**Changes:**
- Replace components with Bahari equivalents
- Update spacing to match wireframe

#### **MonitorScreen.kt** - Field Visits
**Current:**
- Lists monitoring visits
- Uses cards

**Changes:**
- Replace Card with BahariCard
- Update list spacing to 10dp
- Update card internal padding to 12dp

#### **AddPlotScreen.kt** / **AddActivityScreen.kt** - Data Entry Forms
**Current:**
- Uses OutlinedTextField
- Uses standard buttons

**Changes:**
- Wrap OutlinedTextField in BahariInput styling
- Replace buttons with BahariButton
- Update form spacing to match wireframe (10dp gaps)
- Add voice guidance button if applicable

#### **VerificationScreen.kt** - Reports
**Current:**
- Shows verification pipeline
- Has stage indicators (partially implemented)

**Changes:**
- Update stage circles to exact wireframe style
- Use BahariCard for report cards
- Update spacing

#### **CreditsScreen.kt** - Carbon Credits
**Current:**
- Shows credit listings
- Has retirement certificates

**Changes:**
- Update credit cards to BahariCard
- Implement navy gradient card for certificates:
  ```kotlin
  BahariGradientCard(
      colors = CardDefaults.cardColors(
          containerColor = Color(0xFF12263B),
          contentColor = Color(0xFFDFF2FB)
      ),
      border = BorderStroke(1.dp, Color(0xFF2A4356))
  )
  ```

#### **ProfileScreen.kt** - User Profile
**Current:**
- Shows user info
- Shows credentials (partially)

**Changes:**
- Update layout to match Blue ID wireframe card style
- Use BahariCard for credential list
- Add shield icons to verified credentials
- Update spacing

#### **AuthScreen.kt** - Login
**Current:**
- Basic auth flow

**Changes:**
- Consider adding gradient background like wireframe splash
- Update buttons to BahariButton
- Update inputs to BahariInput style

---

## 7. ICON SVG PATH DATA EXTRACTION

For icon conversion, here are the exact SVG paths from the wireframe (sampling key icons):

### Priority Icons Path Data

```xml
<!-- i-home -->
<path d="M3 10.5 12 3l9 7.5V21h-6v-6H9v6H3z"/>

<!-- i-book -->
<path d="M4 19V5a2 2 0 0 1 2-2h14v16H6a2 2 0 0 0-2 2zm0 0a2 2 0 0 0 2 2h14"/>
<path d="M8 7h8M8 11h5"/>

<!-- i-store -->
<path d="M4 9l1.2-5h13.6L20 9M4 9v11h16V9M4 9h16M9.5 20v-6h5v6"/>

<!-- i-wallet -->
<path d="M3 8a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
<circle cx="16.5" cy="13" r="1.3"/>

<!-- i-user -->
<circle cx="12" cy="8" r="4"/>
<path d="M4 21c1-4 4.5-6 8-6s7 2 8 6"/>

<!-- i-fish -->
<path d="M5 12c2.5-3.5 6-5 9-5 3.5 0 6 2.5 7 5-1 2.5-3.5 5-7 5-3 0-6.5-1.5-9-5z"/>
<path d="M5 12 2 8.5v7z"/>
<circle cx="16.5" cy="11" r=".9" fill="currentColor"/>

<!-- i-leaf -->
<path d="M5 19C5 10 10 5 20 4c-1 10-6 15-15 15z"/>
<path d="M5 19C7.5 12 11.5 8 20 4"/>

<!-- i-boat -->
<path d="M4 16h16l-2 4H6z"/>
<path d="M12 3v13M12 5c3 1 5 3 6 6h-6M12 8c-2 .8-3.5 2.5-4 5h4"/>

<!-- i-check -->
<path d="M4.5 12.5 10 18 19.5 6.5"/>

<!-- i-qr -->
<rect x="4" y="4" width="6" height="6" rx="1"/>
<rect x="14" y="4" width="6" height="6" rx="1"/>
<rect x="4" y="14" width="6" height="6" rx="1"/>
<path d="M14 14h3v3h-3zM20 14v.01M14 20h.01M17 20h.01M20 17h.01M20 20h.01"/>

<!-- i-pin -->
<path d="M12 21s-6.5-5.6-6.5-10.5a6.5 6.5 0 1 1 13 0C18.5 15.4 12 21 12 21z"/>
<circle cx="12" cy="10.5" r="2.2"/>

<!-- i-bell -->
<path d="M6 9a6 6 0 0 1 12 0c0 5 2 6 2 6H4s2-1 2-6"/>
<path d="M10 19a2 2 0 0 0 4 0"/>

<!-- i-search -->
<circle cx="11" cy="11" r="6"/>
<path d="m20 20-4.2-4.2"/>

<!-- i-plus -->
<path d="M12 5v14M5 12h14"/>

<!-- i-arr -->
<path d="M4 12h15M13 6l6 6-6 6"/>
```

**Note:** All other icon paths can be extracted similarly from the wireframe HTML source.

---

## 8. GRADIENTS & SPECIAL EFFECTS

### 8.1 Gradient Backgrounds

#### Hero Gradient (Splash/Login)
```kotlin
val heroGradient = Brush.linearGradient(
    0.0f to Color(0xFF082F3F),
    0.55f to Color(0xFF0E7490),
    1.0f to Color(0xFF155E75),
    start = Offset(0f, 0f),
    end = Offset(Float.POSITIVE_INFINITY, Float.POSITIVE_INFINITY)
)
```

#### Blue ID Card Gradient
```kotlin
val blueIdGradient = Brush.linearGradient(
    colorStops = arrayOf(
        0.0f to Color(0xFF12263B),
        1.0f to Color(0xFF0E7490)
    ),
    start = Offset(0f, 0f),
    end = Offset(Float.POSITIVE_INFINITY, Float.POSITIVE_INFINITY)
)
```

#### Wallet Card Gradient
```kotlin
val walletGradient = Brush.linearGradient(
    colorStops = arrayOf(
        0.0f to Color(0xFF12263B),
        1.0f to Color(0xFF1D4ED8)
    ),
    start = Offset(0f, 0f),
    end = Offset(Float.POSITIVE_INFINITY, Float.POSITIVE_INFINITY)
)
```

### 8.2 Dashed Borders

**Wireframe CSS:**
```css
border: 1.5px dashed var(--wire);
background: repeating-linear-gradient(45deg, #f7fafc, #f7fafc 9px, #edf3f8 9px, #edf3f8 18px);
```

**Compose Implementation:**
```kotlin
fun Modifier.dashedBorder(
    color: Color = Color(0xFFB9C6D6),
    strokeWidth: Dp = 1.5.dp,
    dashLength: Dp = 6.dp,
    gapLength: Dp = 4.dp,
    cornerRadius: Dp = 12.dp
): Modifier = this.drawBehind {
    drawRoundRect(
        color = color,
        cornerRadius = CornerRadius(cornerRadius.toPx()),
        style = Stroke(
            width = strokeWidth.toPx(),
            pathEffect = PathEffect.dashPathEffect(
                floatArrayOf(dashLength.toPx(), gapLength.toPx())
            )
        )
    )
}.background(
    brush = Brush.linearGradient(
        colors = listOf(
            Color(0xFFF7FAFC),
            Color(0xFFEDF3F8),
            Color(0xFFF7FAFC),
            Color(0xFFEDF3F8)
        ),
        start = Offset.Zero,
        end = Offset(100f, 100f),
        tileMode = TileMode.Repeated
    ),
    shape = RoundedCornerShape(cornerRadius)
)
```

### 8.3 Box Shadows

**Wireframe CSS:**
```css
box-shadow: 0 18px 40px rgba(15,35,60,.28), inset 0 0 0 2px #24344b;
```

**Compose Note:** Material3 cards use elevation instead of box-shadow. For wireframe-accurate shadows, use:
```kotlin
Card(
    elevation = CardDefaults.cardElevation(defaultElevation = 4.dp),
    // ... other properties
)
```

For specific shadow control, use `Modifier.shadow()`:
```kotlin
Modifier.shadow(
    elevation = 4.dp,
    shape = RoundedCornerShape(12.dp),
    clip = false
)
```

---

## 9. BUILD & TEST COMMANDS

### 9.1 Current Project Setup

**Build System:** Gradle (Android)  
**Language:** Kotlin  
**UI Framework:** Jetpack Compose  
**Architecture:** MVVM with Hilt DI

### 9.2 Build Commands

```bash
# Navigate to android directory
cd /Users/Komarov/Documents/BahariMRV/android

# Clean build
./gradlew clean

# Build debug APK
./gradlew assembleDebug

# Install on connected device
./gradlew installDebug

# Run app on device
adb shell am start -n ke.co.baharilink.baharimrv/.MainActivity

# View logs
adb logcat | grep BahariMRV
```

### 9.3 Layout Inspector

```bash
# Open Android Studio
open -a "Android Studio" /Users/Komarov/Documents/BahariMRV/android

# In Android Studio: Tools > Layout Inspector
# Connect to running app to inspect component dimensions
```

### 9.4 Compose Preview

Add preview annotations to components:
```kotlin
@Preview(showBackground = true)
@Composable
fun BahariCardPreview() {
    BahariMrvTheme {
        BahariCard {
            Text("Preview content")
        }
    }
}
```

---

## 10. IMPLEMENTATION CHECKLIST

### Phase 1: Foundation ✓
- [ ] Update Theme.kt with wireframe colors
- [ ] Update Type.kt with wireframe typography
- [ ] Update Shapes with wireframe radii
- [ ] Add extended color scheme
- [ ] Convert 15 priority icons to vector drawables
- [ ] Create Icons.kt helper file

### Phase 2: Component Library ✓
- [ ] Create BahariComponents.kt
- [ ] Implement BahariCard
- [ ] Implement BahariButton (all variants)
- [ ] Implement BahariChip
- [ ] Implement BahariPill
- [ ] Implement BahariInput
- [ ] Implement BahariIconBackground
- [ ] Implement BahariToast
- [ ] Implement BahariHash
- [ ] Implement BahariAnnotationBadge
- [ ] Implement BahariStatusBadge
- [ ] Create BahariAdvancedComponents.kt
- [ ] Implement BahariGradientCard
- [ ] Implement BahariSwitch
- [ ] Implement BahariProgressBar
- [ ] Implement dashed border modifier

### Phase 3: Navigation ✓
- [ ] Update bottom navigation height to 54dp
- [ ] Update bottom nav icon size to 18dp
- [ ] Update bottom nav text size to 8.5sp
- [ ] Implement module-colored active state
- [ ] Add offline pill to app bar

### Phase 4: Screen Updates ✓
- [ ] Update HomeScreen.kt
- [ ] Update ProjectsScreen.kt
- [ ] Update ProjectDetailScreen.kt
- [ ] Update MonitorScreen.kt
- [ ] Update AddPlotScreen.kt
- [ ] Update AddActivityScreen.kt
- [ ] Update VerificationScreen.kt
- [ ] Update WalletScreen.kt (major redesign)
- [ ] Update CreditsScreen.kt
- [ ] Update ProfileScreen.kt
- [ ] Update AuthScreen.kt

### Phase 5: Icon Completion ✓
- [ ] Convert remaining 25 icons
- [ ] Update all icon usages across screens
- [ ] Verify icon sizes (18dp standard)

### Phase 6: Polish ✓
- [ ] Spacing audit (verify 9dp, 10dp, 12dp patterns)
- [ ] Color audit (verify all hex values match)
- [ ] Typography audit (verify all sp values match)
- [ ] Border radius audit (verify all dp values match)
- [ ] Test on device
- [ ] Side-by-side comparison with wireframe
- [ ] Screenshot comparison
- [ ] Layout Inspector measurement verification

---

## 11. RISK ASSESSMENT & MITIGATION

### 11.1 Risks

#### High Risk
1. **Breaking existing functionality** - Component replacement might break data binding
   - **Mitigation:** Update one screen at a time, test thoroughly before moving to next
   
2. **Icon conversion errors** - SVG to vector drawable conversion might lose details
   - **Mitigation:** Visual comparison of each icon, test at different sizes

#### Medium Risk
3. **Typography sizing issues** - sp values might not scale identically to px
   - **Mitigation:** Use density-independent measurements, test on multiple screen sizes
   
4. **Gradient rendering differences** - CSS gradients vs Compose Brush
   - **Mitigation:** Test gradients on device, adjust angle/color stops as needed

#### Low Risk
5. **Color reproduction** - Slight variations in color rendering
   - **Mitigation:** Use exact hex values, verify with color picker
   
6. **Spacing precision** - Rounding differences between px and dp
   - **Mitigation:** Accept 1-2dp variance, prioritize visual consistency

### 11.2 Rollback Plan

- All changes in feature branch
- Commit after each screen update
- Can revert individual commits if issues arise
- Keep original components available during transition

---

## 12. SUCCESS CRITERIA

### 12.1 Visual Match
- [ ] Colors match wireframe hex values exactly
- [ ] Font sizes match wireframe (with acceptable sp conversion)
- [ ] Spacing matches wireframe (within 1-2dp)
- [ ] Border radii match wireframe
- [ ] Icons match wireframe style (stroke weight, caps, joins)
- [ ] Bottom nav height is 54dp
- [ ] Card padding is 12dp
- [ ] Button heights are 48dp (standard) / 36dp (small)

### 12.2 Functionality Preservation
- [ ] All existing features work
- [ ] Navigation works
- [ ] Data loads correctly
- [ ] Forms submit correctly
- [ ] Offline mode works
- [ ] Sync works
- [ ] No crashes introduced

### 12.3 User Experience
- [ ] App feels cohesive with new design
- [ ] Touch targets meet 48dp minimum
- [ ] Text is readable (contrast ratios met)
- [ ] Interactive elements are obvious
- [ ] Loading states are clear
- [ ] Error states are clear

---

## SUMMARY

**Artifact Path:** `/Users/Komarov/Documents/BahariMRV/.agents/tasks/android-redesign/DESIGN_PLAN.md`

**Key Numbers:**
- **40 icons** to convert to Android vector drawables
- **20+ Compose files** to modify with new components
- **15 color values** extracted from wireframe
- **12 typography sizes** mapped to Compose
- **8 border radius values** for shape system

**Estimated Scope: LARGE**
- 12 days of focused implementation
- Complete visual redesign
- New component library creation
- All screens require updates
- Extensive testing required

**Next Steps:**
1. Review and approve this design plan
2. Set up feature branch for redesign work
3. Begin Phase 1: Foundation (theme, typography, priority icons)
4. Proceed through phases sequentially
5. Test thoroughly at each phase
6. Final side-by-side comparison with wireframe
