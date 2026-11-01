import { useCallback } from 'react';
import { ArrowDownLeft, ArrowUpRight, Download, Landmark, RefreshCw, Scale, WalletCards } from 'lucide-react';
import { ProjectGate } from '../components/ProjectGate';
import { useProject } from '../components/ProjectContext';
import { Button, EmptyTableRow, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatAmount, formatDate, titleCase } from '../lib/format';
import { useApiData } from '../lib/useApiData';

function WalletContent() {
  const { projectId } = useProject();
  const loader = useCallback(async (signal: AbortSignal) => {
    const [wallet, reconciliation] = await Promise.all([api.getWallet(projectId, signal), api.getReconciliation(projectId, signal)]);
    return { wallet, reconciliation };
  }, [projectId]);
  const { data, loading, error, reload } = useApiData(loader, Boolean(projectId));

  return (
    <>
      <PageHeader title="Project wallet" description="Read-only BahariLink financial imports, effective revenue rules, and custody reconciliation." meta={<StatusBadge status="read_only" label="Read-only imports" />} actions={<Button variant="secondary" icon={RefreshCw} onClick={reload} loading={loading}>Refresh imports</Button>} />
      <InlineNotice tone="info" title="Financial operating boundary">BahariMRV does not initiate regulated disbursements in this release. Amounts are fixed-precision decimal values returned by the API; no browser float arithmetic is applied.</InlineNotice>
      {loading && <><div className="wallet-balance-grid">{[1, 2, 3].map((item) => <Panel key={item}><Skeleton rows={2} /></Panel>)}</div><Panel><Skeleton rows={6} /></Panel></>}
      {!loading && error && <StatePanel kind="error" title="Financial imports unavailable" onRetry={reload}><p>{error}</p><p>Previously displayed balances are not presented as current.</p></StatePanel>}
      {!loading && data && <>
        <section className="wallet-hero">
          <div className="wallet-hero-head"><div><WalletCards size={21} /><span>Imported project balances</span></div><StatusBadge status={data.wallet.mode} /></div>
          <div className="wallet-hero-balances"><div><span>Tourism contributions</span><strong>{formatAmount(data.wallet.balances.tourismContributions)}</strong><small>BahariLink read-only import</small></div><div><span>Carbon-credit sales</span><strong>{formatAmount(data.wallet.balances.carbonCreditSales)}</strong><small>Settlement and custody records</small></div><div><span>Community share</span><strong>{formatAmount(data.wallet.balances.communityShare)}</strong><small>Calculated under effective rule</small></div></div>
          <div className="wallet-hero-foot"><span>Last imported {formatDate(data.wallet.lastImportedAt)}</span><span className="mono">Project · {data.wallet.projectId}</span></div>
        </section>

        <div className="wallet-detail-grid">
          <Panel title="Effective revenue rule" description="The configured rule and calculated amounts are displayed together; no universal split is assumed.">
            {data.wallet.revenueRule ? <div className="revenue-rule"><div className="rule-summary"><span className="rule-icon"><Scale size={22} /></span><div><strong>{data.wallet.revenueRule.label}</strong><span>Effective {formatDate(data.wallet.revenueRule.effectiveFrom)}</span></div><strong>{data.wallet.revenueRule.communityPercent}%<small> community</small></strong></div><div className="table-scroll"><table><thead><tr><th>Allocation</th><th>Rule</th><th>Calculated amount</th></tr></thead><tbody>{data.wallet.revenueRule.allocations.map((allocation) => <tr key={allocation.label}><td data-label="Allocation">{allocation.label}</td><td data-label="Rule">{allocation.percent}%</td><td data-label="Amount">{formatAmount(allocation.amount)}</td></tr>)}</tbody></table></div><span className="mono rule-id">Rule · {data.wallet.revenueRule.id}</span></div> : <StatePanel kind="empty" title="No effective revenue rule supplied"><p>Community share cannot be recalculated or explained until the project rule is imported.</p></StatePanel>}
          </Panel>
          <Panel title="Custody reconciliation" description="Registry, ownership, serial, and financial mismatches remain visible.">
            <div className="reconciliation-summary"><span className={`reconciliation-icon reconciliation-${data.reconciliation.status}`}><Landmark size={23} /></span><div><strong>{titleCase(data.reconciliation.status)}</strong><span>Checked {formatDate(data.reconciliation.checkedAt)}</span></div><StatusBadge status={data.reconciliation.status} /></div>
            {data.reconciliation.issues.length ? <div className="reconciliation-issues">{data.reconciliation.issues.map((issue) => <div key={issue.id}><StatusBadge status={issue.severity} /><p>{issue.detail}</p></div>)}</div> : <StatePanel kind={data.reconciliation.status === 'reconciled' ? 'success' : 'empty'} title={data.reconciliation.status === 'reconciled' ? 'No reconciliation exceptions' : 'No issue details available'}><p>{data.reconciliation.status === 'reconciled' ? 'The backend reports that current financial and custody records reconcile.' : 'An unavailable or pending adapter must not be interpreted as reconciled.'}</p></StatePanel>}
          </Panel>
        </div>

        <Panel title="Audit ledger" description="Source, status, timestamp, and references are preserved for every imported entry." action={<span className="count-label">{data.wallet.entries.length} entries</span>}>
          <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Entry</th><th>Source</th><th>Occurred</th><th>Status</th><th className="align-right">Amount</th></tr></thead><tbody>
            {data.wallet.entries.map((entry) => <tr key={entry.id}><td data-label="Entry"><div className="ledger-entry"><span>{entry.kind === 'tourism_contribution' ? <ArrowDownLeft size={16} /> : <ArrowUpRight size={16} />}</span><div><strong>{entry.description}</strong><small>{titleCase(entry.kind)} · <span className="mono">{entry.reference ?? entry.id}</span></small></div></div></td><td data-label="Source">{entry.source}</td><td data-label="Occurred">{formatDate(entry.occurredAt)}</td><td data-label="Status"><StatusBadge status={entry.status} /></td><td data-label="Amount" className="align-right amount-cell">{formatAmount(entry.amount)}</td></tr>)}
            {!data.wallet.entries.length && <EmptyTableRow colSpan={5}>No financial entries have been imported for this project.</EmptyTableRow>}
          </tbody></table></div>
          <div className="ledger-foot"><Download size={14} /><span>Financial evidence exports are generated by authorized proof and reporting workflows.</span></div>
        </Panel>
      </>}
    </>
  );
}

export function WalletPage() {
  return <ProjectGate><WalletContent /></ProjectGate>;
}
