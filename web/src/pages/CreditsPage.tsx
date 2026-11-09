import { useCallback, useMemo, useState, type FormEvent } from 'react';
import { ArrowUpRight, Award, Check, Coins, Leaf, LockKeyhole, RefreshCw, ShieldCheck, ShoppingCart } from 'lucide-react';
import { Button, FormError, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { compareDecimal, isPositiveDecimal, multiplyDecimal } from '../lib/decimal';
import { formatAmount, formatDate, formatQuantity, networkName } from '../lib/format';
import { useApiData } from '../lib/useApiData';
import type { MarketplaceListing, PurchaseResult, RetirementResult } from '../types/api';

function canPurchase(listing: MarketplaceListing): boolean {
  return listing.productType === 'registry_issued' && listing.registry.status === 'reconciled' && listing.mode !== 'unavailable';
}

function RetirementCertificate({ retirement }: { retirement: RetirementResult }) {
  const complete = retirement.status === 'completed';
  return (
    <section className={`retirement-certificate ${complete ? '' : 'certificate-pending'}`} aria-labelledby="retirement-title">
      <div className="certificate-top"><div className="certificate-seal"><Award size={27} /></div><StatusBadge status={retirement.status} /><StatusBadge status={retirement.mode} /></div>
      <span className="certificate-kicker">BAHARIMRV / REGISTRY RETIREMENT</span>
      <h2 id="retirement-title">{complete ? 'Retirement certificate' : 'Retirement reconciliation record'}</h2>
      <p>{complete ? 'Registry retirement and BahariMRV provenance are recorded for this certificate.' : retirement.detail ?? 'This is not a completed certificate. Registry and chain references must reconcile before completion.'}</p>
      <strong className="certificate-quantity">{formatQuantity(retirement.quantityTonnes)}</strong>
      <dl><div><dt>Retirement ID</dt><dd className="mono">{retirement.id}</dd></div><div><dt>Credit batch</dt><dd className="mono">{retirement.batchId}</dd></div><div><dt>Registry serial reference</dt><dd className="mono">{retirement.registrySerialReference ?? 'Pending'}</dd></div><div><dt>Registry certificate</dt><dd className="mono">{retirement.registryCertificateReference ?? 'Pending'}</dd></div><div><dt>{retirement.chainNetwork ? `${networkName(retirement.chainNetwork)} reference` : 'Ledger reference'}</dt><dd className="mono">{retirement.chainReference ?? 'Pending'}</dd></div><div><dt>Retired at</dt><dd>{formatDate(retirement.retiredAt)}</dd></div></dl>
      {retirement.explorerUrl && <a href={retirement.explorerUrl} target="_blank" rel="noreferrer">Inspect ledger provenance <ArrowUpRight size={15} /></a>}
    </section>
  );
}

export function CreditsPage() {
  const loader = useCallback((signal: AbortSignal) => api.getListings(signal), []);
  const { data, loading, error, reload } = useApiData(loader);
  const [selectedId, setSelectedId] = useState('');
  const [quantity, setQuantity] = useState('1');
  const [password, setPassword] = useState('');
  const [purchaseKey, setPurchaseKey] = useState(() => crypto.randomUUID());
  const [purchaseLoading, setPurchaseLoading] = useState(false);
  const [purchaseError, setPurchaseError] = useState('');
  const [purchase, setPurchase] = useState<PurchaseResult | null>(null);
  const [retireBatch, setRetireBatch] = useState('');
  const [retireQuantity, setRetireQuantity] = useState('1');
  const [retireReason, setRetireReason] = useState('');
  const [retirePassword, setRetirePassword] = useState('');
  const [retireKey, setRetireKey] = useState(() => crypto.randomUUID());
  const [retireLoading, setRetireLoading] = useState(false);
  const [retireError, setRetireError] = useState('');
  const [retirement, setRetirement] = useState<RetirementResult | null>(null);

  const listings = useMemo(() => data?.listings ?? [], [data]);
  const selected = listings.find((listing) => listing.id === selectedId) ?? listings.find(canPurchase) ?? listings[0];
  const total = useMemo(() => {
    if (!selected || !isPositiveDecimal(quantity)) return null;
    try { return { amount: multiplyDecimal(quantity, selected.pricePerTonne.amount), currency: selected.pricePerTonne.currency }; }
    catch { return null; }
  }, [quantity, selected]);
  const sandbox = selected?.mode === 'sandbox' || purchase?.mode === 'sandbox' || retirement?.mode === 'sandbox';

  const buy = async (event: FormEvent) => {
    event.preventDefault();
    if (!selected) { setPurchaseError('Select a registry-backed listing.'); return; }
    if (!canPurchase(selected)) { setPurchaseError('This inventory is not reconciled registry-issued credit available for purchase.'); return; }
    if (!isPositiveDecimal(quantity) || compareDecimal(quantity, selected.availableTonnes) > 0) { setPurchaseError(`Enter a quantity greater than zero and no more than ${selected.availableTonnes} tonnes.`); return; }
    if (!password) { setPurchaseError('Confirm your password to authorise this transfer.'); return; }
    setPurchaseLoading(true);
    setPurchaseError('');
    try {
      const receipt = await api.stepUp(password, 'marketplace_purchase');
      const result = await api.purchase({ listingId: selected.id, quantityTonnes: quantity, stepUpCode: receipt.stepUpCode, idempotencyKey: purchaseKey });
      setPurchase(result);
      setRetireBatch(result.batchId);
      setRetireQuantity(result.quantityTonnes);
      setPassword('');
      setPurchaseKey(crypto.randomUUID());
      reload();
    } catch (cause) {
      setPurchaseError(cause instanceof Error ? cause.message : 'Purchase could not be completed.');
      setPassword('');
    }
    finally { setPurchaseLoading(false); }
  };

  const retire = async (event: FormEvent) => {
    event.preventDefault();
    if (!retireBatch.trim() || !retireReason.trim()) { setRetireError('Credit batch and retirement reason are required.'); return; }
    if (!isPositiveDecimal(retireQuantity)) { setRetireError('Retirement quantity must be greater than zero.'); return; }
    if (!retirePassword) { setRetireError('Confirm your password to authorise this retirement.'); return; }
    setRetireLoading(true);
    setRetireError('');
    try {
      const receipt = await api.stepUp(retirePassword, 'credit_retirement');
      const result = await api.retire(retireBatch.trim(), { quantityTonnes: retireQuantity, reason: retireReason.trim(), stepUpCode: receipt.stepUpCode, idempotencyKey: retireKey });
      setRetirement(result);
      setRetirePassword('');
      setRetireKey(crypto.randomUUID());
      reload();
    } catch (cause) { setRetireError(cause instanceof Error ? cause.message : 'Retirement could not be submitted.'); setRetirePassword(''); }
    finally { setRetireLoading(false); }
  };

  return (
    <>
      <PageHeader title="Blue-carbon credits" description="Browse reconciled registry inventory, settle through the enabled adapter, and retire owned serials." meta={<StatusBadge status={sandbox ? 'sandbox' : 'registry'} label={sandbox ? 'Sandbox environment' : 'Registry-backed only'} />} actions={<Button variant="secondary" icon={RefreshCw} onClick={reload} loading={loading}>Refresh inventory</Button>} />
      {sandbox && <InlineNotice tone="info" title="Sandbox transaction boundary">Purchases, settlements, custody changes, and retirements in this environment are test operations. They are not live financial or registry claims.</InlineNotice>}
      {loading && <Panel><Skeleton rows={6} label="Loading reconciled marketplace inventory" /></Panel>}
      {!loading && error && <StatePanel kind="error" title="Marketplace inventory unavailable" onRetry={reload}><p>{error}</p><p>No inventory or price has been substituted for registry-backed data.</p></StatePanel>}
      {!loading && data && <>
        {!listings.length ? <StatePanel kind="empty" title="No registry-backed inventory available"><p>Verified achievements do not automatically create credits. Inventory appears only after external registry issuance and custody reconciliation.</p></StatePanel> : <div className="market-layout">
          <Panel className="market-inventory mw-card" title="Available inventory" description="Availability reconciles registry serials with internal custody, not chain supply alone.">
            <div className="listing-grid">{listings.map((listing) => <button key={listing.id} className={`listing-card mw-card ${selected?.id === listing.id ? 'is-selected' : ''}`} onClick={() => { setSelectedId(listing.id); setPurchase(null); }} aria-pressed={selected?.id === listing.id}>
              <div className="listing-top"><StatusBadge status={listing.mode} /><StatusBadge status={listing.registry.status} label={listing.registry.status === 'reconciled' ? 'Registry reconciled' : listing.registry.status} /></div>
              <div className="listing-project"><span className="listing-leaf"><Leaf size={19} /></span><div><strong>{listing.projectName}</strong><span>{listing.vintage} vintage · {listing.registry.name}</span></div></div>
              <div className="listing-supply"><strong>{listing.availableTonnes}</strong><span>tCO₂e available</span></div>
              <div className="co-benefits">{listing.coBenefits.map((benefit) => <span key={benefit}><Check size={12} />{benefit}</span>)}</div>
              <div className="listing-price"><span>{listing.productType === 'registry_issued' ? 'Issued credit price' : 'Forward product · not issued credit'}</span><strong>{formatAmount(listing.pricePerTonne)}<small> / tCO₂e</small></strong></div>
            </button>)}</div>
          </Panel>
          <Panel className="purchase-panel mw-card" title="Purchase issued credits" description="The enabled payment and custody adapter determines the returned status.">
            {selected ? <form onSubmit={buy} className="purchase-form">
              <div className="purchase-selection"><span>Selected batch</span><strong>{selected.projectName}</strong><small className="mono">{selected.batchId}</small></div>
              <div className="field"><label className="mw-ttl-xs" htmlFor="purchase-quantity">Quantity</label><div className="input-unit"><input id="purchase-quantity" inputMode="decimal" className="mw-inp" value={quantity} onChange={(event) => setQuantity(event.target.value)} required /><span>tCO₂e</span></div></div>
              <div className="purchase-total"><span>Calculated total <small>Decimal arithmetic</small></span><strong>{total ? formatAmount(total) : 'Enter a valid quantity'}</strong></div>
              {selected.registry.reference && <div className="registry-reference mw-row"><ShieldCheck size={16} /><span className="grow">Registry reference</span><strong className="mono">{selected.registry.reference}</strong></div>}
              <div className="field"><label className="mw-ttl-xs" htmlFor="purchase-password">Confirm your password</label><p className="field-help">Mints a single-use ticket for this purchase only.</p><input id="purchase-password" type="password" autoComplete="current-password" className="mw-inp" value={password} onChange={(event) => setPassword(event.target.value)} required /></div>
              <FormError message={purchaseError} />
              <Button icon={ShoppingCart} loading={purchaseLoading} disabled={!canPurchase(selected)} className="mw-btn mw-btn--g">{selected.mode === 'sandbox' ? 'Submit sandbox purchase' : 'Purchase registry credits'}</Button>
              {!canPurchase(selected) && <p className="field-help">Purchase is blocked until this is reconciled, registry-issued inventory with an enabled adapter.</p>}
              <p className="secure-caption"><LockKeyhole size={13} />Idempotent request · no card details collected here</p>
            </form> : <StatePanel kind="empty" title="Select a listing"><p>Choose an inventory item to review its price and settlement state.</p></StatePanel>}
          </Panel>
        </div>}
        {purchase && <section className="transaction-result" aria-live="polite"><div><Coins size={22} /><div><span>{purchase.mode === 'sandbox' ? 'Sandbox purchase response' : 'Purchase response'}</span><h2>{purchase.status === 'confirmed' ? 'Custody event confirmed' : 'Purchase requires attention'}</h2></div><StatusBadge status={purchase.status} /></div><dl><div><dt>Purchase ID</dt><dd className="mono">{purchase.id}</dd></div><div><dt>Quantity</dt><dd>{formatQuantity(purchase.quantityTonnes)}</dd></div><div><dt>Total</dt><dd>{formatAmount(purchase.total)}</dd></div><div><dt>Custody reference</dt><dd className="mono">{purchase.custodyReference ?? 'Pending'}</dd></div><div><dt>Settlement reference</dt><dd className="mono">{purchase.settlementReference ?? 'Pending'}</dd></div></dl></section>}

        <div className="retirement-layout">
          <Panel title="Retire owned credits" description="Retirement locks serials, waits for registry confirmation, and records corresponding provenance.">
            <form className="retire-form" onSubmit={retire}>
              <div className="form-grid form-grid-2"><div className="field"><label htmlFor="retire-batch">Owned credit batch ID</label><input id="retire-batch" value={retireBatch} onChange={(event) => setRetireBatch(event.target.value)} required /></div><div className="field"><label htmlFor="retire-quantity">Quantity</label><div className="input-unit"><input id="retire-quantity" inputMode="decimal" value={retireQuantity} onChange={(event) => setRetireQuantity(event.target.value)} required /><span>tCO₂e</span></div></div></div>
              <div className="field"><label htmlFor="retire-reason">Retirement reason</label><textarea id="retire-reason" rows={3} value={retireReason} onChange={(event) => setRetireReason(event.target.value)} placeholder="Purpose and beneficiary statement for the registry record" required /></div>
              <div className="field"><label htmlFor="retire-password">Confirm your password</label><p className="field-help">Mints a single-use ticket for this retirement only.</p><input id="retire-password" type="password" autoComplete="current-password" value={retirePassword} onChange={(event) => setRetirePassword(event.target.value)} required /></div>
              <FormError message={retireError} />
              <Button icon={Award} loading={retireLoading}>Submit retirement</Button>
            </form>
          </Panel>
          {retirement ? <RetirementCertificate retirement={retirement} /> : <div className="certificate-placeholder"><Award size={28} /><h2>No retirement selected</h2><p>A certificate appears only from the backend response. Partial registry or chain failure produces a reconciliation record, never a false completed certificate.</p></div>}
        </div>
      </>}
    </>
  );
}
