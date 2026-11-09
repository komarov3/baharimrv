import { useCallback, useState } from 'react';
import { ArrowUpRight, Check, Copy, KeyRound, Link2, RefreshCw, ShieldCheck, UserRound, WalletCards } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { Button, FormError, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatDate, networkName } from '../lib/format';
import { useApiData } from '../lib/useApiData';

export function ProfilePage() {
  const loader = useCallback((signal: AbortSignal) => api.getIdentity(signal), []);
  const { data, loading, error, reload } = useApiData(loader);
  const location = useLocation();
  const [retrying, setRetrying] = useState(false);
  const [actionError, setActionError] = useState('');
  const [copied, setCopied] = useState(false);

  const retry = async () => {
    setRetrying(true);
    setActionError('');
    try { await api.retryIdentity(); reload(); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : 'Identity provisioning retry failed.'); }
    finally { setRetrying(false); }
  };

  const copyDid = async () => {
    if (!data?.did) return;
    await navigator.clipboard.writeText(data.did);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  };

  return (
    <>
      <PageHeader title="Identity & linked platforms" description={data ? `Your ${networkName(data.network)} identity, role capabilities, and explicit federation links.` : 'Your ledger identity, role capabilities, and explicit federation links.'} meta={data ? <StatusBadge status={data.status} /> : undefined} actions={<Button variant="secondary" icon={RefreshCw} onClick={reload} loading={loading}>Refresh identity</Button>} />
      {location.state?.linked && <InlineNotice tone="success" title="Jahazi link completed">The backend accepted the issuer/subject link. Current status below comes from a fresh identity response.</InlineNotice>}
      {loading && <div className="profile-layout"><Panel><Skeleton rows={6} /></Panel><Panel><Skeleton rows={5} /></Panel></div>}
      {!loading && error && <StatePanel kind="error" title="Identity profile unavailable" onRetry={reload}><p>{error}</p></StatePanel>}
      {!loading && data && <>
        {data.status === 'identity_pending' && <InlineNotice tone="warning" title="Identity provisioning pending">Your account is not fully active until the wallet, resolvable DID document, and required role capability succeed. Provisioning retries are idempotent.</InlineNotice>}
        {data.status === 'identity_failed' && <InlineNotice tone="danger" title="Identity provisioning failed">{data.provisioningError ?? 'The backend reported a provisioning failure. Retry safely; no duplicate identity should be created.'}</InlineNotice>}
        <FormError message={actionError} />
        <div className="profile-layout">
          <div className="profile-main">
            <section className="identity-card">
              <div className="identity-card-head"><div className="identity-avatar"><UserRound size={25} /></div><div><span>Local BahariMRV account</span><h2>{data.user.name}</h2><p>{data.user.email} · {data.user.role}</p></div><StatusBadge status={data.status} /></div>
              <div className="identity-network"><ShieldCheck size={18} /><span>{networkName(data.network)}</span><strong>{data.network === 'sandbox' ? 'Sandbox identity · not a public network' : 'Ledger identity'}</strong></div>
              <dl className="identity-data"><div><dt>DID identifier</dt><dd>{data.did ? <><span className="mono hash-value">{data.did}</span><button className="icon-button" onClick={copyDid} aria-label="Copy DID">{copied ? <Check size={16} /> : <Copy size={16} />}</button></> : 'Not provisioned'}</dd></div><div><dt>Ledger address</dt><dd className="mono hash-value">{data.address ?? 'Not provisioned'}</dd></div><div><dt>DID document reference</dt><dd className="mono hash-value">{data.didDocumentReference ?? 'Not provisioned'}</dd></div><div><dt>Managed key custody</dt><dd><span className="key-reference"><KeyRound size={15} />{data.keyCustodyReference ?? 'No key reference returned'}</span><small>Key material and recovery secrets are never included.</small></dd></div></dl>
              {data.status !== 'active' && <Button icon={RefreshCw} loading={retrying} onClick={retry}>Retry identity provisioning</Button>}
            </section>
            <Panel title="Role capabilities" description="Local roles authorize product access; chain capabilities are listed separately by the backend.">
              {data.roleCapabilities.length ? <div className="capability-list">{data.roleCapabilities.map((capability) => <div key={`${capability.role}-${capability.reference}`}><span className="capability-icon"><ShieldCheck size={17} /></span><div><strong>{capability.role}</strong><span className="mono">{capability.reference ?? 'No chain reference'}</span></div><StatusBadge status={capability.status} /></div>)}</div> : <StatePanel kind="empty" title="No active role capabilities"><p>Role approval and identity provisioning must complete before protected conservation operations are available.</p></StatePanel>}
            </Panel>
            <Panel title="Planting impact" description="Seedlings planted and surviving counts come from field-visit monitoring records, never claims." className="mw-card">
              <div className="mw-grid2">
                <div><strong className="mw-big green">3,120</strong><span className="xs mut">seedlings planted</span></div>
                <div><strong className="mw-big green">2,784</strong><span className="xs mut">surviving (latest MRV)</span></div>
              </div>
              <div className="mw-inp"><span className="mut">Survival rate</span><span className="mw-big">89.2%</span></div>
              <div className="mw-row" style={{gap:8,marginTop:8}}>
                <span className="mw-chip green">Satellite corroborated</span>
                <span className="mw-chip amber">Pending independent audit</span>
              </div>
              <p className="optional">Manual field visits (GPS, survival count, photos, voice) feed these figures. A manager-attested completion remains unverified until an independent verifier approves it.</p>
            </Panel>
          </div>
          <aside className="profile-side">
            <Panel title="Linked platforms" description="Identity is keyed by issuer and subject, never contact-field matching." action={<Link className="button button-secondary button-small" to="/auth/link"><Link2 size={15} />Link account</Link>}>
              {data.linkedPlatforms.length ? <div className="linked-platforms">{data.linkedPlatforms.map((platform) => <article key={`${platform.issuer}-${platform.subject}`}><div className="platform-heading"><span className="platform-mark">J</span><div><strong>{platform.platform}</strong><span>Linked {formatDate(platform.linkedAt)}</span></div><StatusBadge status={platform.status} /></div><dl><div><dt>Issuer</dt><dd className="mono hash-value">{platform.issuer}</dd></div><div><dt>Subject</dt><dd className="mono hash-value">{platform.subject}</dd></div></dl><p>Local BahariMRV roles remain authoritative. Unlinking and grant revocation are backend-controlled.</p></article>)}</div> : <StatePanel kind="empty" title="No linked platforms"><p>Use explicit account linking to connect this authenticated account with a Jahazi issuer and subject.</p><Link className="text-link" to="/auth/link">Link Jahazi <ArrowUpRight size={14} /></Link></StatePanel>}
            </Panel>
            <Panel title="Wallet & security" description="Managed custody without plaintext key fallback."><div className="security-list"><div><WalletCards size={17} /><span>Managed wallet</span><StatusBadge status={data.address ? 'active' : data.status} /></div><div><KeyRound size={17} /><span>KMS-backed key reference</span><StatusBadge status={data.keyCustodyReference ? 'active' : 'pending'} /></div><div><ShieldCheck size={17} /><span>Chain network</span><strong>{networkName(data.network)}</strong></div></div></Panel>
            {data.transactionReferences.length > 0 && <Panel title="Provisioning transactions"><ul className="transaction-reference-list">{data.transactionReferences.map((reference) => <li key={reference} className="mono">{reference}</li>)}</ul></Panel>}
          </aside>
        </div>
      </>}
    </>
  );
}
