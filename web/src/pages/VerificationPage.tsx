import { useCallback, useMemo, useState, type FormEvent } from 'react';
import { ArrowDownToLine, ArrowRight, CheckCircle2, FileWarning, Inbox, RefreshCw, Search, Send, ShieldCheck } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { ProvenanceTimeline } from '../components/ProvenanceTimeline';
import { Button, FormError, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatDate } from '../lib/format';
import { useApiData } from '../lib/useApiData';
import type { VerificationQueueItem } from '../types/api';

export function VerificationPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activityId = searchParams.get('activity') ?? '';
  const [activityDraft, setActivityDraft] = useState(activityId);
  const queueLoader = useCallback((signal: AbortSignal) => api.getVerificationQueue(signal), []);
  const { data: queue, loading: queueLoading, error: queueError, reload: reloadQueue } = useApiData(queueLoader);
  const loader = useCallback((signal: AbortSignal) => api.getProvenance(activityId, signal), [activityId]);
  const { data, loading, error, reload } = useApiData(loader, Boolean(activityId));
  const [decision, setDecision] = useState<'approve' | 'reject' | 'request_corrections'>('request_corrections');
  const [notes, setNotes] = useState('');
  const [methodologyVersion, setMethodologyVersion] = useState('');
  const [password, setPassword] = useState('');
  const [decisionError, setDecisionError] = useState('');
  const [decisionNotice, setDecisionNotice] = useState('');
  const [decisionLoading, setDecisionLoading] = useState(false);
  const [anchorLoading, setAnchorLoading] = useState(false);
  // One key per decision attempt: a lost response can be retried without the
  // backend recording a second signed decision.
  const [attemptKey, setAttemptKey] = useState(() => crypto.randomUUID());

  const openQueueItem = (item: VerificationQueueItem) => {
    setActivityDraft(item.activityId);
    setSearchParams({ activity: item.activityId });
    setMethodologyVersion(item.methodologyVersion);
    setAttemptKey(crypto.randomUUID());
    setDecisionNotice('');
    setDecisionError('');
  };

  const loadActivity = (event: FormEvent) => {
    event.preventDefault();
    const id = activityDraft.trim();
    if (id) setSearchParams({ activity: id });
  };

  const submitDecision = async (event: FormEvent) => {
    event.preventDefault();
    if (!data?.activity.verificationRequestId) { setDecisionError('No verification request ID was returned for this activity.'); return; }
    if (!notes.trim() || !methodologyVersion.trim()) { setDecisionError('Decision notes and methodology version are required.'); return; }
    if (!password) { setDecisionError('A signed decision requires recent password confirmation.'); return; }
    setDecisionLoading(true);
    setDecisionError('');
    setDecisionNotice('');
    const requestId = attemptKey;
    try {
      // The ticket is single-use and bound to this action, so it is minted
      // immediately before the decision instead of being cached.
      const receipt = await api.stepUp(password, 'verification_decision');
      const result = await api.submitDecision(data.activity.verificationRequestId, {
        decision,
        notes: notes.trim(),
        methodologyVersion: methodologyVersion.trim(),
        stepUpCode: receipt.stepUpCode,
        idempotencyKey: requestId,
      });
      setDecisionNotice(result.attestationReference
        ? `Recorded: ${result.status} · attestation ${result.attestationReference} · anchoring ${result.provenanceStatus ?? 'not started'}.`
        : `The backend recorded decision status: ${result.status}. Refreshing the evidence pipeline.`);
      setNotes('');
      setPassword('');
      setAttemptKey(crypto.randomUUID());
      reloadQueue();
      reload();
    } catch (cause) {
      setDecisionError(cause instanceof Error ? cause.message : 'The decision could not be recorded.');
      setPassword('');
    } finally { setDecisionLoading(false); }
  };

  const retryAnchor = async () => {
    if (!activityId) return;
    setAnchorLoading(true);
    setDecisionError('');
    try { const result = await api.retryAnchor(activityId); setDecisionNotice(`Anchor retry accepted with status: ${result.status}. Confirmation still requires chain readback.`); reload(); }
    catch (cause) { setDecisionError(cause instanceof Error ? cause.message : 'Anchor retry failed.'); }
    finally { setAnchorLoading(false); }
  };

  const pendingCount = useMemo(() => (queue?.items ?? []).filter((item) => item.status === 'requested' || item.status === 'in_review').length, [queue]);

  return (
    <>
      <PageHeader title="Verification pipeline" description="Review evidence ownership, independent decisions, and provenance without collapsing distinct milestones." meta={<StatusBadge status="independent" label="Independent review" />} actions={<Button variant="secondary" icon={RefreshCw} onClick={() => { reloadQueue(); reload(); }} disabled={loading}>Refresh evidence</Button>} />

      <Panel
        title="Assigned verification queue"
        description="Only requests this account is assigned to appear. Nothing here can be decided without password confirmation."
        action={<span className="count-label">{queueLoading ? 'Loading' : `${pendingCount} awaiting a decision`}</span>}
      >
        {queueLoading && <Skeleton rows={3} label="Loading the verification queue" />}
        {!queueLoading && queueError && <StatePanel kind="error" title="Verification queue unavailable" onRetry={reloadQueue}><p>{queueError}</p></StatePanel>}
        {!queueLoading && queue && !queue.items.length && <StatePanel kind="empty" title="No verification requests assigned"><p>Requests arrive here after a manager claims completion and the activity has been submitted for independent review.</p></StatePanel>}
        {!queueLoading && queue && queue.items.length > 0 && (
          <div className="table-scroll">
            <table>
              <thead><tr><th>Project &amp; plot</th><th>Requested</th><th>Records</th><th>Evidence</th><th>State</th><th aria-label="Action" /></tr></thead>
              <tbody>
                {queue.items.map((item) => (
                  <tr key={item.id}>
                    <td data-label="Project"><strong>{item.projectName}</strong><small className="block-help mono">{item.plotCode}</small></td>
                    <td data-label="Requested">{formatDate(item.requestedAt)}</td>
                    <td data-label="Records">{item.recordsCount}{item.unsignedRecords > 0 && <span className="count-label"> · {item.unsignedRecords} unsigned</span>}</td>
                    <td data-label="Evidence">{item.evidenceCount}</td>
                    <td data-label="State"><StatusBadge status={item.status} /></td>
                    <td data-label="Action" className="align-right"><button className="row-action" type="button" onClick={() => openQueueItem(item)}>Review <ArrowRight size={14} /></button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Panel className="lookup-panel"><form className="lookup-form" onSubmit={loadActivity}><div><label htmlFor="activity-lookup">Or open a known activity</label><p>The backend enforces verifier assignment and methodology rules on every decision.</p></div><div className="lookup-input"><input id="activity-lookup" value={activityDraft} onChange={(event) => setActivityDraft(event.target.value)} placeholder="Activity identifier" required /><Button type="submit" variant="secondary" icon={Search}>Load evidence</Button></div></form></Panel>
      {!activityId && <StatePanel kind="empty" title="Select an assigned conservation activity"><Inbox size={20} /><p>Its four verification stages, documents, exceptions, and full provenance history will load here.</p></StatePanel>}
      {loading && <div className="verification-layout"><Panel><Skeleton rows={6} /></Panel><Panel><Skeleton rows={6} /></Panel></div>}
      {!loading && error && <StatePanel kind="error" title="Verification evidence unavailable" onRetry={reload}><p>{error}</p></StatePanel>}
      {!loading && data && <>
        <div className="record-context"><div><span>Activity</span><strong className="mono">{data.activity.id}</strong></div><div><span>Project</span><strong>{data.activity.projectName}</strong></div><div><span>Current status</span><StatusBadge status={data.activity.status} /></div></div>
        <Panel title="Required stages" description="A required stage cannot be skipped through an interface or administrator edit.">
          <ol className="verification-stages">{data.stages.map((stage, index) => <li key={stage.id} className={`stage-${stage.status}`}>
            <div className="stage-rail"><span>{stage.status === 'complete' ? <CheckCircle2 size={18} /> : index + 1}</span></div>
            <div className="stage-body"><div className="stage-heading"><div><h3>{stage.label}</h3><p>{stage.owner}</p></div><StatusBadge status={stage.status} /></div>
              <dl className="stage-meta"><div><dt>Timestamp</dt><dd>{formatDate(stage.timestamp)}</dd></div><div><dt>Evidence references</dt><dd>{stage.evidenceReferences.length ? stage.evidenceReferences.join(', ') : 'None supplied'}</dd></div></dl>
              {stage.exception && <p className="stage-exception"><FileWarning size={15} />{stage.exception}</p>}
              {stage.nextAction && <p className="stage-next"><strong>Next action</strong>{stage.nextAction}</p>}
            </div>
          </li>)}</ol>
          {!data.stages.length && <StatePanel kind="empty" title="No verification stages returned"><p>The activity may not yet be submitted for independent verification.</p></StatePanel>}
        </Panel>

        <div className="verification-layout">
          <div className="verification-main">
            <Panel title="Evidence documents" description="Restricted documents open only through authorized, expiring URLs.">
              {data.documents.length ? <div className="document-list">{data.documents.map((document) => <article key={document.id} className="document-row"><div className="document-icon">{document.status === 'missing' ? <FileWarning size={19} /> : <ShieldCheck size={19} />}</div><div><strong>{document.name}</strong><span>{document.kind.replaceAll('_', ' ')} {document.version ? `· v${document.version}` : ''} · {formatDate(document.updatedAt)}</span></div><StatusBadge status={document.status} />{document.downloadUrl && <a className="icon-button" href={document.downloadUrl} target="_blank" rel="noreferrer" aria-label={`Open ${document.name}`}><ArrowDownToLine size={17} /></a>}</article>)}</div> : <StatePanel kind="empty" title="No documents returned"><p>Required project design, monitoring, and co-benefit documents have not been attached to this activity.</p></StatePanel>}
            </Panel>
            <Panel title="Achievement & custody timeline" description="Early evidence, manager completion, and independent verification retain their distinct meanings."><ProvenanceTimeline events={data.events} /></Panel>
          </div>
          <aside className="decision-column">
            <Panel title="Record verifier decision" description="Your verifier role, assignment, and methodology version are checked by the backend.">
              <form className="decision-form" onSubmit={submitDecision}>
                <div className="field"><label htmlFor="decision">Decision</label><select id="decision" value={decision} onChange={(event) => setDecision(event.target.value as typeof decision)}><option value="request_corrections">Request corrections</option><option value="approve">Approve independently</option><option value="reject">Reject achievement</option></select></div>
                <div className="field"><label htmlFor="decision-methodology">Methodology version</label><input id="decision-methodology" value={methodologyVersion} onChange={(event) => setMethodologyVersion(event.target.value)} required /></div>
                <div className="field"><label htmlFor="decision-notes">Evidence-based notes</label><textarea id="decision-notes" rows={5} value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="Identify the evidence, exceptions, and reasoning for this decision" required /></div>
                <div className="field">
                  <label htmlFor="decision-password">Confirm your password</label>
                  <p className="field-help">Mints a single-use ticket for this decision only. It is never stored in this app.</p>
                  <input id="decision-password" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
                </div>
                <FormError message={decisionError} />
                {decisionNotice && <InlineNotice tone="success" title="Backend response recorded">{decisionNotice}</InlineNotice>}
                <Button icon={Send} loading={decisionLoading} disabled={!data.activity.verificationRequestId}>Submit signed decision</Button>
                {!data.activity.verificationRequestId && <p className="field-help">No active verification request was supplied; decisions are unavailable.</p>}
              </form>
            </Panel>
            <Panel title="Anchor recovery" description="Retry preserves the original commitment. A submitted transaction is not confirmed until readback matches."><Button variant="secondary" icon={RefreshCw} loading={anchorLoading} onClick={retryAnchor}>Retry failed anchor</Button></Panel>
            <InlineNotice tone="warning" title="Approval is not registry issuance">An independently verified achievement does not create carbon credits. Registry issuance and reconciliation remain separate stages.</InlineNotice>
          </aside>
        </div>
      </>}
    </>
  );
}
