import { useState, type FormEvent } from 'react';
import { ArrowUpRight, Check, Download, FileArchive, FileJson, FileText, Link2, PackageCheck, ShieldCheck } from 'lucide-react';
import { Link } from 'react-router-dom';
import { ProofQr } from '../components/ProofQr';
import { ProvenanceTimeline } from '../components/ProvenanceTimeline';
import { Button, FormError, InlineNotice, PageHeader, Panel, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatDate, networkName } from '../lib/format';
import type { ProofDocument, ProofFormat } from '../types/api';

const formatDetails = {
  pdf: { label: 'PDF document', description: 'Human-readable proof from the same snapshot', icon: FileText },
  json: { label: 'JSON manifest', description: 'Canonicalizable machine-readable claims', icon: FileJson },
  zip: { label: 'ZIP evidence package', description: 'PDF, JSON, public evidence & instructions', icon: FileArchive },
};

export function ProofsPage() {
  const [achievementId, setAchievementId] = useState('');
  const [proof, setProof] = useState<ProofDocument | null>(null);
  const [history, setHistory] = useState<ProofDocument[]>([]);
  const [loading, setLoading] = useState(false);
  const [download, setDownload] = useState<ProofFormat | null>(null);
  const [error, setError] = useState('');

  const generate = async (event: FormEvent) => {
    event.preventDefault();
    if (!achievementId.trim()) { setError('Enter an achievement ID.'); return; }
    setLoading(true);
    setError('');
    const id = achievementId.trim();
    try {
      const generated = await api.generateProof(id);
      setProof(generated);
      // The register is re-read after generating so a new version and the record
      // it supersedes both appear, exactly as the backend reports them.
      const register = await api.listProofs(id);
      setHistory(register.proofs);
    }
    catch (cause) { setProof(null); setError(cause instanceof Error ? cause.message : 'Proof generation failed.'); }
    finally { setLoading(false); }
  };

  const downloadProof = async (proofFormat: ProofFormat) => {
    if (!proof) return;
    setDownload(proofFormat);
    setError('');
    try {
      const blob = await api.downloadProof(proof.id, proofFormat);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `baharimrv-${proof.id}-v${proof.version}.${proofFormat}`;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'The proof download failed.'); }
    finally { setDownload(null); }
  };

  return (
    <>
      <PageHeader title="Proof center" description="Generate a versioned proof from current evidence, attestations, provenance, and registry state." meta={<StatusBadge status="on_demand" label="On-demand verification" />} />
      <div className="proof-generator-layout">
        <Panel title="Generate achievement proof" description="The backend reloads the current record and independently reads the ledger it anchors to before creating the snapshot.">
          <form onSubmit={generate} className="proof-generate-form">
            <div className="field"><label htmlFor="achievement-id">Achievement ID</label><input id="achievement-id" value={achievementId} onChange={(event) => setAchievementId(event.target.value)} placeholder="Authorized achievement identifier" required /></div>
            <p className="field-help">The proof type is decided by the achievement's own state: a manager attestation renders a completion claim, an approved and independently attested activity renders a verified achievement. Download formats come from the snapshot that was built.</p>
            <FormError message={error} />
            <Button icon={PackageCheck} loading={loading}>Generate current proof</Button>
          </form>
        </Panel>
        <div className="proof-principles"><h2>A proof says exactly what was checked</h2><div><span><Check size={17} /></span><p><strong>Completion claim</strong>Manager-attested <code>achieved_unverified</code>; never displays a verified-achievement badge.</p></div><div><span><ShieldCheck size={17} /></span><p><strong>Verified achievement</strong>Separate independent attestation and confirmed chain commitment.</p></div><div><span><Link2 size={17} /></span><p><strong>Permanent public URL</strong>Corrections create new versions; superseded URLs remain available.</p></div></div>
      </div>
      {!proof && !loading && <StatePanel kind="empty" title="No proof generated in this session"><p>Enter an achievement ID above. Rejected or unverified activities cannot produce an official verified-achievement proof.</p></StatePanel>}
      {loading && <div className="proof-loading" role="status"><span className="proof-loading-line" /><PackageCheck size={26} /><h2>Building a consistent proof snapshot</h2><p>Loading current attestations, evidence hashes, provenance, registry state, and independent chain checks.</p></div>}
      {!loading && proof && <div className="proof-document">
        <section className="proof-summary">
          <div className="proof-summary-top"><div><span>BAHARIMRV / ACHIEVEMENT PROOF</span><h2>{proof.proofType === 'verified_achievement' ? 'Independently verified achievement' : 'Manager-attested completion claim'}</h2></div><StatusBadge status={proof.status} /><StatusBadge status={proof.proofType === 'verified_achievement' ? 'verified' : 'achieved_unverified'} /></div>
          <p>{proof.claimSummary}</p>
          {proof.sandboxNotice && <InlineNotice tone="info" title="Sandbox evidence boundary">{proof.sandboxNotice}</InlineNotice>}
          <div className="proof-identifiers"><div><span>Proof ID</span><strong className="mono">{proof.id}</strong></div><div><span>Version</span><strong>v{proof.version}</strong></div><div><span>Generated</span><strong>{formatDate(proof.generatedAt)}</strong></div><div><span>Project</span><strong>{proof.projectName}</strong></div></div>
          {proof.status === 'superseded' && <InlineNotice tone="warning" title="This proof is superseded">{proof.supersededBy ? `A newer proof ${proof.supersededBy} replaces this version.` : 'A newer proof version replaces this snapshot.'}</InlineNotice>}
        </section>
        <div className="proof-export-grid">
          <Panel title="Download proof package" description="Every format is served by the backend from the same proof snapshot.">
            <div className="download-list">{proof.availableFormats.map((item) => { const Icon = formatDetails[item].icon; return <button key={item} onClick={() => void downloadProof(item)} disabled={Boolean(download)}><Icon size={20} /><span><strong>{formatDetails[item].label}</strong><small>{download === item ? 'Downloading from API…' : formatDetails[item].description}</small></span><Download size={17} /></button>; })}</div>
            <Link className="text-link" to={`/proof/${encodeURIComponent(proof.id)}`}>Open public independent verification <ArrowUpRight size={14} /></Link>
          </Panel>
          <ProofQr proofId={proof.id} />
        </div>
        <div className="proof-data-grid">
          <Panel title="Attestation & chain" description="Only confirmed references are displayed as confirmed.">
            <dl className="proof-data-list"><div><dt>Issuer DID</dt><dd className="mono hash-value">{proof.issuerDid ?? 'Not supplied'}</dd></div><div><dt>Verifier organization</dt><dd>{proof.verifierAttestation?.organization ?? 'Not applicable'}</dd></div><div><dt>Verifier decision</dt><dd>{proof.verifierAttestation ? `${proof.verifierAttestation.decision} · ${formatDate(proof.verifierAttestation.timestamp)}` : 'No independent attestation'}</dd></div><div><dt>Chain network</dt><dd>{proof.chain ? networkName(proof.chain.network) : 'No confirmed chain record'}</dd></div><div><dt>Transaction reference</dt><dd className="mono hash-value">{proof.chain?.transactionReference ?? 'Pending or unavailable'}</dd></div><div><dt>Anchored commitment</dt><dd className="mono hash-value">{proof.chain?.commitment ?? 'Pending or unavailable'}</dd></div><div><dt>Registry references</dt><dd>{proof.registryReferences.length ? proof.registryReferences.join(', ') : 'None · no registry issuance implied'}</dd></div></dl>
            {proof.chain?.explorerUrl && <a className="text-link" href={proof.chain.explorerUrl} target="_blank" rel="noreferrer">Open configured explorer <ArrowUpRight size={14} /></a>}
          </Panel>
          <Panel title="Evidence manifest" description="Restricted evidence remains excluded from public packages.">
            {proof.evidenceManifest.length ? <div className="manifest-list">{proof.evidenceManifest.map((evidence) => <div key={`${evidence.name}-${evidence.sha256}`}><FileText size={16} /><div><strong>{evidence.name}</strong><span className="mono hash-value">{evidence.sha256}</span></div><StatusBadge status={evidence.access} /></div>)}</div> : <StatePanel kind="empty" title="No public evidence entries"><p>The proof snapshot contains no evidence entries permitted for this view.</p></StatePanel>}
          </Panel>
        </div>
        {history.length > 1 && <Panel title="Proof version register" description="Corrections create a new version; superseded snapshots stay retrievable.">
          <div className="table-scroll"><table><thead><tr><th>Version</th><th>Proof type</th><th>Status</th><th>Generated</th><th>Manifest hash</th></tr></thead><tbody>
            {history.map((item) => <tr key={item.id}>
              <td data-label="Version">{item.id === proof.id ? <strong>v{item.version}</strong> : `v${item.version}`}</td>
              <td data-label="Proof type">{item.proofType === 'verified_achievement' ? 'Verified achievement' : 'Completion claim'}</td>
              <td data-label="Status"><StatusBadge status={item.status} /></td>
              <td data-label="Generated">{formatDate(item.generatedAt)}</td>
              <td data-label="Manifest hash"><span className="mono hash-value">{item.manifestHash}</span></td>
            </tr>)}
          </tbody></table></div>
        </Panel>}
        <Panel title="Chronological provenance" description="The original claim and subsequent verification or custody events remain distinct."><ProvenanceTimeline events={proof.events} /></Panel>
      </div>}
    </>
  );
}
