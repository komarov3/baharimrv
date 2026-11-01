import { useCallback, useState } from 'react';
import { ArrowLeft, ArrowUpRight, CheckCircle2, FileCheck2, LockKeyhole, RefreshCw, ShieldCheck, XCircle } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { ProofQr } from '../components/ProofQr';
import { ProvenanceTimeline } from '../components/ProvenanceTimeline';
import { Brand, Button, DataPair, InlineNotice, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatDate, networkName } from '../lib/format';
import { useApiData } from '../lib/useApiData';
import type { ProofVerification } from '../types/api';

export function PublicProofPage() {
  const { id = '' } = useParams();
  const loader = useCallback((signal: AbortSignal) => api.getPublicProof(id, signal), [id]);
  const { data, loading, error, reload } = useApiData(loader, Boolean(id));
  const [verification, setVerification] = useState<ProofVerification | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [verifyError, setVerifyError] = useState('');
  const proof = data?.proof;
  const checks = verification?.checks ?? data?.verification?.checks;
  const overall = verification?.overallStatus ?? data?.verification?.overallStatus;

  const verify = async () => {
    setVerifying(true);
    setVerifyError('');
    try { setVerification(await api.verifyPublicProof(id)); }
    catch (cause) { setVerifyError(cause instanceof Error ? cause.message : 'Independent verification could not be completed.'); }
    finally { setVerifying(false); }
  };

  return (
    <div className="public-proof-page">
      <header className="public-header"><Brand /><nav><span><LockKeyhole size={14} />Public, privacy-minimized view</span><Link to="/login">Operator sign in <ArrowUpRight size={14} /></Link></nav></header>
      <main className="public-main">
        <Link to="/login" className="back-link"><ArrowLeft size={15} />BahariMRV</Link>
        <div className="public-title"><div><span className="section-label">INDEPENDENT PROOF VERIFICATION</span><h1>Check the evidence,<br />not just the badge.</h1><p>This public view compares the proof snapshot with live chain and attestation checks. No sign-in is required.</p></div>{id && <ProofQr proofId={id} />}</div>
        {loading && <div className="public-loading"><Skeleton rows={7} label="Loading public proof and verification state" /></div>}
        {!loading && error && <StatePanel kind="error" title="Public proof unavailable" onRetry={reload}><p>{error}</p><p>Check the permanent proof ID or ask the proof issuer for a current link. No validity is inferred from a failed lookup.</p></StatePanel>}
        {!loading && proof && <>
          <section className="public-proof-summary">
            <div className="public-summary-top"><div className="public-proof-icon"><FileCheck2 size={25} /></div><div><span>{proof.proofType === 'verified_achievement' ? 'Conservation achievement proof' : 'Completion claim proof'}</span><h2>{proof.projectName}</h2></div><StatusBadge status={proof.status} /><StatusBadge status={proof.proofType === 'verified_achievement' ? 'verified' : 'achieved_unverified'} /></div>
            <p>{proof.claimSummary}</p>
            {proof.sandboxNotice && <InlineNotice tone="info" title="Sandbox evidence boundary">{proof.sandboxNotice}</InlineNotice>}
            {proof.status !== 'anchored' && <InlineNotice tone={proof.status === 'failed' || proof.status === 'revoked' ? 'danger' : 'warning'} title={`Proof status: ${proof.status}`}>{proof.status === 'superseded' ? `This version is not current${proof.supersededBy ? `; version ${proof.supersededBy} supersedes it` : ''}.` : 'The current proof status must be considered alongside each independent check below.'}</InlineNotice>}
            <dl className="public-data-grid"><DataPair label="Proof ID" value={proof.id} mono /><DataPair label="Version" value={`v${proof.version}`} /><DataPair label="Generated" value={formatDate(proof.generatedAt)} /><DataPair label="Issuer DID" value={proof.issuerDid ?? 'Not supplied'} mono /><DataPair label="Chain network" value={proof.chain ? networkName(proof.chain.network) : 'No confirmed chain record'} /><DataPair label="Transaction reference" value={proof.chain?.transactionReference ?? 'Pending or unavailable'} mono /><DataPair label="Permanent URL" value={data?.url ? <a className="mono hash-value" href={data.url}>{data.url}</a> : 'Not supplied by this proof'} /><DataPair label="Registry references" value={proof.registryReferences.length ? proof.registryReferences.join(', ') : 'None'} /></dl>
          </section>

          <section className="verification-checks" aria-labelledby="checks-title">
            <div className="checks-heading"><div><span className="section-label">LIVE CHECKS</span><h2 id="checks-title">Independent verification</h2><p>Each result is reported separately. Unsupported external checks remain unavailable.</p></div><Button icon={RefreshCw} loading={verifying} onClick={verify}>Run current checks</Button></div>
            {overall && <div className={`overall-result overall-${overall}`} role="status">{overall === 'valid' ? <CheckCircle2 size={25} /> : <XCircle size={25} />}<div><strong>{overall === 'valid' ? 'All required checks passed' : overall === 'invalid' ? 'One or more required checks failed' : `Verification ${overall}`}</strong><span>Checked {formatDate(verification?.checkedAt ?? data?.verification?.checkedAt)}</span></div><StatusBadge status={overall} /></div>}
            {verifyError && <InlineNotice tone="danger" title="Current checks could not be completed">{verifyError}</InlineNotice>}
            {checks?.length ? <div className="check-list">{checks.map((check) => <article key={check.key}><span className={`check-icon check-${check.status}`}>{check.status === 'passed' ? <CheckCircle2 size={19} /> : check.status === 'failed' ? <XCircle size={19} /> : <ShieldCheck size={19} />}</span><div><strong>{check.label}</strong><p>{check.detail}</p></div><StatusBadge status={check.status} /></article>)}</div> : <StatePanel kind="empty" title="No verification results in this snapshot"><p>Run current checks to ask the backend to recompute supported hashes, query {networkName(proof.modes.chain)}, and validate attestation status.</p></StatePanel>}
          </section>

          <div className="public-proof-grid">
            <section className="public-section"><h2>Evidence manifest</h2><p>Only public evidence hashes are exposed. Restricted media and precise sensitive locations stay private.</p>{proof.evidenceManifest.length ? <div className="manifest-list">{proof.evidenceManifest.map((evidence) => <div key={evidence.sha256}><FileCheck2 size={16} /><div><strong>{evidence.name}</strong><span className="mono hash-value">{evidence.sha256}</span></div><StatusBadge status="public" /></div>)}</div> : <StatePanel kind="empty" title="No public evidence objects"><p>The proof can still expose permitted chain and attestation checks without releasing restricted evidence.</p></StatePanel>}</section>
            <section className="public-section"><h2>Verifier attestation</h2><dl className="proof-data-list"><div><dt>Organization</dt><dd>{proof.verifierAttestation?.organization ?? 'Not applicable to this proof type'}</dd></div><div><dt>Decision</dt><dd>{proof.verifierAttestation?.decision ?? 'No independent decision claimed'}</dd></div><div><dt>Timestamp</dt><dd>{formatDate(proof.verifierAttestation?.timestamp)}</dd></div><div><dt>Verifier DID</dt><dd className="mono hash-value">{proof.verifierAttestation?.did ?? 'Not supplied'}</dd></div></dl>{proof.chain?.explorerUrl && <a className="text-link" href={proof.chain.explorerUrl} target="_blank" rel="noreferrer">Inspect configured ledger reference <ArrowUpRight size={14} /></a>}</section>
          </div>
          <section className="public-section public-timeline"><h2>Provenance history</h2><p>Completion claims, independent verification, corrections, and custody changes are not merged into a single status.</p><ProvenanceTimeline events={proof.events} /></section>
          <footer className="public-proof-footer"><ShieldCheck size={19} /><p><strong>What “valid” means here</strong>The backend reports valid only when every required schema, evidence, chain, issuer, attestation, registry, and supersession check passes. A transaction hash alone is not proof of restoration.</p></footer>
        </>}
      </main>
      <footer className="public-footer"><Brand /><span>Public proof ID · <code>{id}</code></span></footer>
    </div>
  );
}
