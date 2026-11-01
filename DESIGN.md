---
name: BahariMRV
description: Evidence-led conservation operations and verification
colors:
  primary: "#079b76"
  primary-deep: "#08775e"
  navy: "#12263b"
  ink: "#142434"
  muted: "#607080"
  canvas: "#f4f7f8"
  surface: "#ffffff"
  line: "#dce5e7"
  success-soft: "#e9f8f2"
  warning: "#b7791f"
typography:
  display:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "28px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  label:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "11px"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "0.08em"
rounded:
  sm: "8px"
  md: "12px"
  lg: "18px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
    rounded: "{rounded.sm}"
    padding: "12px 18px"
  button-primary-hover:
    backgroundColor: "{colors.primary-deep}"
    textColor: "{colors.surface}"
    rounded: "{rounded.sm}"
    padding: "12px 18px"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "16px"
---

# Design System: BahariMRV

## Overview

**Creative North Star: "The Field Evidence Ledger"**

The interface combines the supplied mobile conservation workflows with a calm operational web console. White work surfaces, fine blue-gray borders, compact data rows, and decisive emerald actions keep attention on evidence and next steps. Navy is reserved for financial proof and high-trust summaries. The system is dense where operators scan and spacious where they make decisions.

**Key Characteristics:**
- Evidence status is always visible and never implied by decoration.
- Mobile screens preserve the supplied C1-C4 hierarchy.
- Web uses structured work queues and split views rather than oversized dashboard cards.
- Motion communicates synchronization, verification, and completion only.

## Colors

Emerald is the action and verified-state color. Navy anchors financial and proof surfaces. Neutral blue-grays maintain legibility in bright field conditions and dense web workflows.

### Primary
- **Field Emerald** ({colors.primary}): primary actions, active navigation, positive verified states.
- **Deep Emerald** ({colors.primary-deep}): pressed and hover states.

### Neutral
- **Ledger Navy** ({colors.navy}): retirement certificates, proof summaries, high-trust financial panels.
- **Survey Ink** ({colors.ink}): primary text and measurements.
- **Coastal Gray** ({colors.muted}): secondary text with accessible contrast.
- **Field Canvas** ({colors.canvas}): web application background.
- **Evidence White** ({colors.surface}): forms, tables, and work surfaces.

**The Honest State Rule.** Color never converts pending, unavailable, estimated, or unverified information into a verified-looking state.

## Typography

**Display Font:** Inter with system sans fallbacks  
**Body Font:** Inter with system sans fallbacks  
**Label/Mono Font:** Inter for labels; system monospace only for hashes, identifiers, and canonical evidence data

One workhorse family supports compact field reading and predictable web density. Measurements use weight and tabular numerals rather than decorative display type.

### Hierarchy
- **Display** (700, 28px, 1.2): project and page titles.
- **Title** (650, 18px, 1.3): workflow stages and panels.
- **Body** (400, 14px, 1.5): operational copy and values.
- **Label** (700, 11px, 0.08em): short section labels and status metadata, not repeated decorative eyebrows.

## Layout

Compact mobile uses a single column, sticky context header, and five-destination bottom navigation. Android uses Material 3 navigation and window insets. Web uses a 232px sidebar, contextual top bar, and a twelve-column work area; below 900px it collapses to mobile navigation. Spacing follows 4, 8, 16, 24, and 32px rhythms. Tables and proof data may exceed normal prose measure.

## Elevation & Depth

Surfaces are flat at rest and separated by fine borders and tonal layers. Shadows are reserved for navigation drawers, dialogs, and transient overlays. No colored glows or decorative floating-card stacks.

## Shapes

Controls use gentle 8px rounding, cards 12px, and large proof or wallet panels 18px. Dashed borders indicate capture and upload zones. Pills are limited to statuses, filters, and compact metadata.

## Components

### Buttons
- Filled emerald for the single primary action, outlined for secondary actions, text buttons for low-emphasis navigation.
- Minimum 44px web and 48dp Android targets with visible keyboard focus and disabled/loading states.

### Chips
- Compact tonal labels for anchored, pending, failed, disputed, unverified, and sandbox states.
- Status always includes text or an accessible icon label.

### Cards / Containers
- White, fine-bordered, and internally grouped by task. Avoid nesting cards inside cards.
- Wallet and retirement summaries may use the navy treatment from the supplied wireframe.

### Inputs / Fields
- White background, blue-gray stroke, 8px corners, clear labels, units inside the field layout, and specific error recovery text.

### Navigation
- Mobile preserves Home, Monitor, Credits, Wallet, Profile.
- Web uses task-based project navigation and a public-proof route outside the authenticated shell.

### Proof Timeline
- A vertical evidence sequence with distinct completion and independent-verification events. Each event exposes actor, time, status, and chain reference.

## Do's and Don'ts

### Do:
- **Do** preserve the supplied C1-C4 information hierarchy and compact field workflow.
- **Do** label measured, estimated, claimed, verified, pending, unavailable, and sandbox values explicitly.
- **Do** use real chain and registry states in every proof surface.

### Don't:
- **Don't** introduce plastic-waste, EPR, or unrelated wireframe-module visuals.
- **Don't** use decorative dashboards that obscure evidence status or next actions.
- **Don't** present sample data as live project, registry, satellite, or financial data.
