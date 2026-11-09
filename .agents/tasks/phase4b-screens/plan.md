# Implementation Plan — Phase 4b: 9 Android screens across 5 feature modules

Worktree: `/Users/Komarov/Documents/Nanga/.worktrees/phase4b-screens/android/`
Build: `cd /Users/Komarov/Documents/Nanga/.worktrees/phase4b-screens/android && ./gradlew assembleDebug`
Authoritative spec: `/Users/Komarov/Documents/Nanga/NANGA_WIREFRAMES.html`

## Design decisions (grounded in code read)

1. **ViewModel data source.** The domain repositories (`MarketRepository`, `ScoreRepository`, `LoanRepository`, `BatchRepository`, `UserRepository`, `CatchRepository`) are interfaces with NO implementations and NO Hilt bindings; `core/data` has no repository impls and `AppModule` states bindings are deferred to Phase 4. `NangaApiService` also has no endpoints for wallet transactions, officer verify/ledger, or loan applications. The acceptance criteria are `assembleDebug` success + correct rendering + navigation, not live data. **Decision:** each `@HiltViewModel` exposes `StateFlow<UiState>` (sealed `Loading/Success/Error`) seeded with representative in-memory sample data in `init`, structured so a real repository can replace the seeding later. This keeps Phase 4b scoped to the screens.
2. **Hilt.** `@HiltViewModel` + `StateFlow`. ViewModels take no constructor deps (no repo to inject yet), so the existing empty `FeatureModule.kt` files need no `@Provides`. The Hilt + KSP plugins are already on every feature module.
3. **Navigation wiring.** Pass navigation lambdas (`onBack`, `onSellCatch`, `onNavigateToLoans`, `onNavigateToAggregate`, `onNavigateToBatchDetail`, …) from `NangaNavHost` into composables — idiomatic Compose Navigation, keeps screens free of `NavController`.
4. **Score ring.** Compose `Canvas` + `drawArc` (stroke, `StrokeCap.Round`) on a `Box`; track arc in `BorderLight`, progress arc in `GreenMid`; percentage + tier centered in a `Box` overlay. No third-party chart lib.
5. **Officer nav.** Update `officerTabs` from 3 to 4: Home/Verify/Batches/Profile. Batches tab → CoopLedger (O-05). Add routes `officer/verify`, `officer/aggregate`, `officer/batch/{batchId}`, `officer/ledger`; remove the old `officer/batch`. Flow: Verify(O-02) → Aggregate(O-03) → Detail(O-04); Ledger(O-05) batch row → Detail(O-04).
6. **Loans.** Create `LoanApplicationScreen.kt` (F-12); update `NangaNavHost` to call it (replacing `LoansScreen`); delete the `LoansScreen.kt` stub.
7. **Marketplace.** Add `SellCatchScreen.kt` (F-11) + route `marketplace/sell`. Replace `MarketplaceScreen.kt` stub (F-10).
8. **Color discipline.** Feature code uses only `NangaTheme` tokens (`Color.kt` vals) and `MaterialTheme.colorScheme`/`MaterialTheme.shapes`/`MaterialTheme.typography`. No hardcoded hex, no `Color(0x…)`.
9. **Corner radius.** Max 12dp — use `MaterialTheme.shapes` (`medium`=8, `large`=12) or `RoundedCornerShape` ≤ 12dp.

## Verification baseline (today)
- `./gradlew assembleDebug` from the android dir is the only project build gate; feature modules declare no test deps, so there are no unit tests to run.
- Current HEAD `87583e6` builds (Phase 3 skeleton).

---

- [ ] 1. **feature:marketplace — MarketplaceScreen (F-10).** Replace stub with `TealGradientHeader(title="Marketplace", subtitle="42 active listings", actions=Sell button)`, a search `OutlinedTextField`/card, "Live bids" section (rows: avatar + species/kg + buyer + price/kg in GreenDeep + time/OPEN badge), "My listings" section (rows with `AnchorConfirmedChip` for on-chain). Add `MarketplaceViewModel` (`@HiltViewModel`, `StateFlow<MarketplaceUiState>` with live bids + my listings sample data) and `MarketplaceUiState`/row data classes. Screen takes `onSellCatch: () -> Unit` and `viewModel: MarketplaceViewModel = hiltViewModel()`.
      Files: `feature/marketplace/src/main/java/com/nanga/feature/marketplace/MarketplaceScreen.kt`, `MarketplaceViewModel.kt`, `MarketplaceUiState.kt` (new), `di/FeatureModule.kt` (unchanged).
      Verify: `./gradlew :feature:marketplace:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 2. **feature:marketplace — SellCatchScreen (F-11) + nav.** Create `SellCatchScreen.kt`: back-arrow `TealGradientHeader`("Sell Catch"/"Escrow-protected"), "Select catch to sell" with a highlighted selected card (border GreenMid, "Selected" bronze badge), asking-price input, "Or accept open bid" buyer card with Accept button, escrow note card (GreenLight bg, GreenDeep text, lock icon), "List for sale" button. Add `SellCatchViewModel` (`StateFlow<SellCatchUiState>` with user's anchored catches sample + selected catch + asking price; `submitListing()` flips state). Add route `SELL_CATCH="marketplace/sell"` and `Routes.sellCatch()` in `NangaNavHost`; register `composable(SELL_CATCH){ SellCatchScreen(onBack={popBackStack()}, onListed={popBackStack()}) }`; wire `MarketplaceScreen` Sell action → `onSellCatch`.
      Files: `feature/marketplace/.../SellCatchScreen.kt`, `SellCatchViewModel.kt`, `SellCatchUiState.kt` (new); `app/src/main/java/com/nanga/app/navigation/NangaNavHost.kt`.
      Verify: `./gradlew assembleDebug` — BUILD SUCCESSFUL; nav graph compiles.

- [ ] 3. **feature:wallet — WalletScreen (F-05).** Replace stub: `TealGradientHeader`("Wallet", subtitle row with "KSh 18,450 / Available balance" and "M-Pesa linked / +254 7XX"), Withdraw + Statement buttons row, month toggle (This/Last/All time), "Transactions" list with avatar + title + date + amount (+green `GreenDeep` / −red `ErrorRed`). Add `WalletViewModel` (`StateFlow<WalletUiState>` with balance + month + transactions sample) and `WalletUiState`/`Transaction` data classes.
      Files: `feature/wallet/.../WalletScreen.kt`, `WalletViewModel.kt`, `WalletUiState.kt` (new).
      Verify: `./gradlew :feature:wallet:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 4. **feature:score — ScoreScreen (F-06) with Canvas ring.** Replace stub: `TealGradientHeader`("Baharini Score"), Canvas ring (Box with `Canvas` drawing track arc `BorderLight` + progress arc `GreenMid` via `drawArc` with `StrokeCap.Round`, `sweepAngle` from score%, startAngle -90°), centered score number + tier label, tier track (Bronze→Gold) gradient progress bar, "Score components" card with 4 progress bars (catch consistency, loan repayment, BMU compliance, community) using `GreenDeep`/`Amber` fills, "Apply for a loan" CTA button. Add `ScoreViewModel` (`StateFlow<ScoreUiState>` with score/tier/components sample) and data classes. Screen takes `onNavigateToLoans: () -> Unit`.
      Files: `feature/score/.../ScoreScreen.kt`, `ScoreViewModel.kt`, `ScoreUiState.kt` (new).
      Verify: `./gradlew :feature:score:assembleDebug` — BUILD SUCCESSFUL; ring uses `drawArc` (no chart lib import).

- [ ] 5. **feature:score — wire Score CTA to Loans.** In `NangaNavHost`, change `composable(Routes.SCORE){ ScoreScreen() }` to pass `onNavigateToLoans = { navController.navigate(Routes.LOANS) }`.
      Files: `app/src/main/java/com/nanga/app/navigation/NangaNavHost.kt`.
      Verify: `./gradlew assembleDebug` — BUILD SUCCESSFUL.

- [ ] 6. **feature:loans — LoanApplicationScreen (F-12) + nav.** Create `LoanApplicationScreen.kt`: back-arrow `TealGradientHeader`("Loan Application"), advisory score card (green gradient bg: score + tier + max eligible/rate), loan amount input, purpose dropdown (`ExposedDropdownMenuBox`), repayment term toggle (1/3/6 months), repayment schedule card (monthly payment + total interest auto-computed from amount/term/rate), "Apply for KSh N" button. Add `LoanViewModel` (`StateFlow<LoanUiState>` with score + recommendations + amount/term/purpose; computes schedule; `applyForLoan()`) and data classes. In `NangaNavHost` replace `composable(Routes.LOANS){ LoansScreen() }` with `LoanApplicationScreen(onBack={popBackStack()})`; delete `LoansScreen.kt`; remove its import.
      Files: `feature/loans/.../LoanApplicationScreen.kt`, `LoanViewModel.kt`, `LoanUiState.kt` (new); `app/.../NangaNavHost.kt`; delete `feature/loans/.../LoansScreen.kt`.
      Verify: `./gradlew assembleDebug` — BUILD SUCCESSFUL.

- [ ] 7. **feature:officer — BatchVerifyScreen (O-02).** Create `BatchVerifyScreen.kt`: back-arrow header ("Batch Verify"/"N catches selected", bronze "N sel." badge), checklist rows (checkbox ✓ GreenMid / ✗ red for flagged, avatar, fisher + species/kg, "GPS ✓ · Photo ✓" status, `AnchorConfirmedChip`), flagged rows in `ErrorRed`, Officer DID attestation card (BronzeTint bg), "Approve N catches → Anchor to IOTA" primary button + "Reject flagged catch" ghost button. Add `OfficerViewModel` (`StateFlow<OfficerUiState>` pending verify queue sample) and data classes. Screen takes `onBack` + `onApproveToAggregate`.
      Files: `feature/officer/.../BatchVerifyScreen.kt`, `OfficerViewModel.kt`, `OfficerUiState.kt` (new).
      Verify: `./gradlew :feature:officer:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 8. **feature:officer — BatchAggregateScreen (O-03).** Create `BatchAggregateScreen.kt`: header ("Create Batch"/"Group verified catches"), batch name input, "Select verified catches" list (checkboxes, avatars, "⛓ Verified · location"), running totals card (GreenLight bg: count + kg total + est. value), "Create batch → Anchor to IOTA" button. Add `BatchViewModel` (`StateFlow<BatchUiState>` with verified catches + selected set + running totals; `createBatch()`). Screen takes `onBack` + `onCreatedBatch: (batchId) -> Unit`.
      Files: `feature/officer/.../BatchAggregateScreen.kt`, `BatchViewModel.kt`, `BatchUiState.kt` (new).
      Verify: `./gradlew :feature:officer:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 9. **feature:officer — BatchDetailScreen (O-04).** Create `BatchDetailScreen.kt`: header ("Batch Detail"/batch id), teal-gradient batch header card (id · name · 3 stats: catches/total/value), catch list with per-catch status buttons — pending → "Mark aggregated" (teal `TealMid`) → "Mark sold" (bronze `BronzeDeep`) → "Done" badge; bulk "Mark all aggregated" / "Mark all sold" row. Extend `BatchViewModel` (or add detail state) with per-catch mark operations + bulk. Screen takes batchId arg + `onBack`.
      Files: `feature/officer/.../BatchDetailScreen.kt`; extend `BatchViewModel.kt`/`BatchUiState.kt`.
      Verify: `./gradlew :feature:officer:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 10. **feature:officer — CoopLedgerScreen (O-05).** Create `CoopLedgerScreen.kt`: header ("Cooperative Ledger"/"Malindi BMU · Nov 2026" + export icon), 3-tab toggle (Catches/Batches/Fishers), summary stats card (4 metrics: catches/weight/batches/sold%), batch list with `StatusBadge` (Sold/Aggregated), top fishers with tier badges, "Export CSV" secondary button. Add `LedgerViewModel` (`StateFlow<LedgerUiState>` 3-tab cooperative history sample + selected tab).
      Files: `feature/officer/.../CoopLedgerScreen.kt`, `LedgerViewModel.kt`, `LedgerUiState.kt` (new).
      Verify: `./gradlew :feature:officer:assembleDebug` — BUILD SUCCESSFUL.

- [ ] 11. **feature:officer — officer nav wiring.** In `NangaNavHost`: update `officerTabs` to 4 (Home→`OFFICER`, Verify→`OFFICER_VERIFY`, Batches→`OFFICER_LEDGER`, Profile→`PROFILE`); add routes `OFFICER_VERIFY="officer/verify"`, `OFFICER_AGGREGATE="officer/aggregate"`, `OFFICER_BATCH_DETAIL="officer/batch/{batchId}"` (+ `officerBatchDetail(id)`), `OFFICER_LEDGER="officer/ledger"`; remove old `OFFICER_BATCH="officer/batch"`; register composables wiring Verify→Aggregate (onApproveToAggregate), Aggregate→Detail (onCreatedBatch → `officerBatchDetail(id)`), Ledger→Detail (batch row → `officerBatchDetail(id)`); add Verify icon to officer bottom bar (`Icons.Default.Check`).
      Files: `app/src/main/java/com/nanga/app/navigation/NangaNavHost.kt`.
      Verify: `./gradlew assembleDebug` — BUILD SUCCESSFUL; officer bottom bar shows 4 tabs.

- [ ] 12. **Full build + integration verification.** Run `./gradlew assembleDebug` and confirm BUILD SUCCESSFUL; confirm no hardcoded `Color(0x…)` in feature code and no chart-lib imports in score; confirm nav graph resolves all routes (Market→Sell, Score→Loans, Officer Verify→Aggregate→Detail, Ledger→Detail).
      Files: whole android project.
      Verify: `./gradlew assembleDebug` — BUILD SUCCESSFUL.

## Digest

| File | Block to change | Covers/verifies |
|---|---|---|
| `feature/marketplace/.../MarketplaceScreen.kt` | `@Composable fun MarketplaceScreen(onSellCatch: () -> Unit, viewModel: MarketplaceViewModel = hiltViewModel())` (replaces `fun MarketplaceScreen()`) | F-10 bid/listing rows; `:feature:marketplace:assembleDebug` |
| `feature/marketplace/.../MarketplaceViewModel.kt` | new `@HiltViewModel class MarketplaceViewModel` → `StateFlow<MarketplaceUiState>` | seeded sample data |
| `feature/marketplace/.../SellCatchScreen.kt` | new `@Composable fun SellCatchScreen(onBack, onListed, viewModel)` | F-11 |
| `feature/marketplace/.../SellCatchViewModel.kt` | new `@HiltViewModel` | submitListing() |
| `feature/wallet/.../WalletScreen.kt` | replaces `fun WalletScreen()` | F-05 +/− amounts |
| `feature/wallet/.../WalletViewModel.kt` | new `@HiltViewModel` | balance + txns |
| `feature/score/.../ScoreScreen.kt` | replaces `fun ScoreScreen()`; adds `Canvas { drawArc }` + `onNavigateToLoans` | F-06 ring via drawArc |
| `feature/score/.../ScoreViewModel.kt` | new `@HiltViewModel` | score/tier/components |
| `feature/loans/.../LoanApplicationScreen.kt` | new `@Composable fun LoanApplicationScreen(onBack, viewModel)` | F-12 schedule auto-compute |
| `feature/loans/.../LoanViewModel.kt` | new `@HiltViewModel` | applyForLoan() |
| `feature/loans/.../LoansScreen.kt` | DELETE | removed from nav |
| `feature/officer/.../BatchVerifyScreen.kt` | new | O-02 checklist + DID card |
| `feature/officer/.../BatchAggregateScreen.kt` | new | O-03 totals card |
| `feature/officer/.../BatchDetailScreen.kt` | new | O-04 mark aggregated/sold |
| `feature/officer/.../CoopLedgerScreen.kt` | new | O-05 3-tab + stats |
| `feature/officer/.../OfficerViewModel.kt` | new `@HiltViewModel` | verify queue |
| `feature/officer/.../BatchViewModel.kt` | new `@HiltViewModel` | create/detail/mark |
| `feature/officer/.../LedgerViewModel.kt` | new `@HiltViewModel` | 3-tab history |
| `feature/officer/.../OfficerScreen.kt` | DELETE (stub) or leave unused | replaced by 4 screens |
| `app/.../NangaNavHost.kt` | `Routes` (+sell/verify/aggregate/batch-detail/ledger), `officerTabs` 4 tabs, `composable(...)` calls passing nav lambdas | nav wiring |
| `core/ui/.../*` | UNCHANGED — reuse TealGradientHeader, AnchorConfirmedChip, StatusBadge, EmptyState | tokens/components |

Verified today: `./gradlew assembleDebug` builds at HEAD `87583e6`; no feature module tests exist.
