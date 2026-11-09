import { useCallback } from 'react';
import { AlertTriangle, ArrowRight, ClipboardCheck, FileText, Leaf, Plus, ShieldCheck, WalletCards } from 'lucide-react';
import { Link } from 'react-router-dom';
import { PlotMap } from '../components/PlotMap';
import { ProjectGate } from '../components/ProjectGate';
import { useProject } from '../components/ProjectContext';
import { EmptyTableRow, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatAmount } from '../lib/format';
import { useApiData } from '../lib/useApiData';

function DashboardContent() {
  const { projectId, projectName } = useProject();
  const loader = useCallback((signal: AbortSignal) => api.getDashboard(projectId, signal), [projectId]);
  const { data, loading, error, reload } = useApiData(loader, Boolean(projectId));

  return (
    <>
      <PageHeader
        title={data?.project.name ?? projectName ?? 'Project dashboard'}
        description={data ? `${data.project.siteCount} ${data.project.siteCount === 1 ? 'site' : 'sites'} · ${data.project.methodology}` : 'Live project metrics and evidence status from the BahariMRV API.'}
        meta={<StatusBadge status="operate" label="Operate" />}
        actions={<><Link className="button button-secondary" to="/app/proofs"><FileText size={17} />Reports & proofs</Link><Link className="button button-primary" to="/app/monitor"><Plus size={17} />Field visit</Link></>}
      />
      {loading && <div className="dashboard-grid"><Panel className="span-8"><Skeleton rows={5} /></Panel><Panel className="span-4"><Skeleton rows={5} /></Panel><Panel className="span-12"><Skeleton rows={3} /></Panel></div>}
      {!loading && error && <StatePanel kind="error" title="Project dashboard unavailable" onRetry={reload}><p>{error}</p><p>Values from a previous session are not substituted for current project data.</p></StatePanel>}
      {!loading && data && <div className="dashboard-content">
        <div className="dashboard-grid">
          <Panel className="span-8 map-panel" title="Restoration area" description="Select a plot to inspect its current evidence status."><PlotMap plots={data.plots} ndvi={data.ndvi} /></Panel>
          <Panel className="span-4 project-pulse" title="Project pulse" description="Units and evidence labels come from the selected methodology period.">
            {data.metrics.filter((metric) => !(metric.key === 'survival' && data.project.conservationType && data.project.conservationType !== 'mangrove')).length ? <dl className="metric-list">{data.metrics.filter((metric) => !(metric.key === 'survival' && data.project.conservationType && data.project.conservationType !== 'mangrove')).map((metric) => <div key={metric.key} className="metric-item"><dt>{metric.label}<StatusBadge status={metric.status} /></dt><dd>{metric.value}<span>{metric.unit}</span></dd>{metric.methodologyPeriod && <small>{metric.methodologyPeriod}</small>}</div>)}</dl> : <StatePanel kind="empty" title="No metrics reported"><p>The project has no dashboard metrics yet. Complete monitoring to populate measured and estimated outcomes.</p></StatePanel>}
          </Panel>
        </div>

        {data.wallet && <section className="wallet-strip mw-card mw-card.hl" aria-labelledby="dashboard-wallet-title">
          <div className="wallet-strip-heading"><span><WalletCards size={19} />Project wallet</span><StatusBadge status={data.wallet.mode} /></div>
          <div className="wallet-balances"><div className="mw-inp"><span className="mut">Tourism contributions</span><strong className="mw-big">{formatAmount(data.wallet.balances.tourismContributions)}</strong></div><div className="mw-inp"><span className="mut">Carbon-credit sales</span><strong className="mw-big green">{formatAmount(data.wallet.balances.carbonCreditSales)}</strong></div><div className="mw-inp"><span className="mut">Community share</span><strong className="mw-big">{formatAmount(data.wallet.balances.communityShare)}</strong></div></div>
          <div className="wallet-strip-foot"><span>{data.wallet.revenueRule ? `${data.wallet.revenueRule.label} · ${data.wallet.revenueRule.communityPercent}% community` : 'Revenue rule not supplied'} · Read-only imports</span><Link to="/app/wallet">Open ledger <ArrowRight size={14} /></Link></div>
        </section>}

        <div className="dashboard-grid lower-dashboard">
          <Panel className="span-7" title="Attention required" description="Exceptions stay visible until their source system resolves them." action={<span className="count-label">{data.alerts.length} open</span>}>
            {data.alerts.length ? <div className="alert-list">{data.alerts.map((alert) => <article key={alert.id} className={`alert-row alert-${alert.severity}`}><AlertTriangle size={18} /><div><strong>{alert.title}</strong><p>{alert.detail}</p>{alert.actionHref?.startsWith('/') && <Link to={alert.actionHref}>{alert.actionLabel ?? 'Review'} <ArrowRight size={13} /></Link>}</div><StatusBadge status={alert.severity} /></article>)}</div> : <StatePanel kind="success" title="No active project alerts"><p>The API returned no current survival, evidence, anchor, monitoring, or registry exceptions.</p></StatePanel>}
          </Panel>
          <Panel className="span-5 workflow-panel" title="Evidence workflow" description="Move from field records to independent checks without skipping a required stage.">
            <nav className="workflow-links" aria-label="Project workflows">
              <Link to="/app/monitor"><span className="workflow-icon"><Leaf size={19} /></span><span><strong>Capture field evidence</strong><small>GPS, survival, media & offline queue</small></span><ArrowRight size={17} /></Link>
              <Link to="/app/verification"><span className="workflow-icon"><ShieldCheck size={19} /></span><span><strong>Review verification</strong><small>Satellite, audit, registry & provenance</small></span><ArrowRight size={17} /></Link>
              <Link to="/app/proofs"><span className="workflow-icon"><ClipboardCheck size={19} /></span><span><strong>Generate a proof</strong><small>PDF, JSON or permitted evidence package</small></span><ArrowRight size={17} /></Link>
            </nav>
            <InlineNotice title="Two distinct milestones" tone="info">A manager-attested completion remains unverified. Independent approval creates a separate achievement event.</InlineNotice>
          </Panel>
        </div>

        <Panel title="Project ledger coverage" description="Current integration coverage for this project.">
          <div className="table-scroll"><table><thead><tr><th>Evidence source</th><th>Availability</th><th>Last reference</th></tr></thead><tbody>
            <tr><td data-label="Source">Field GPS plots</td><td data-label="Availability"><StatusBadge status={data.plots.length ? 'measured' : 'pending'} label={data.plots.length ? 'Records available' : 'No records'} /></td><td data-label="Reference">{data.plots.length} plots returned</td></tr>
            <tr><td data-label="Source">Satellite NDVI</td><td data-label="Availability"><StatusBadge status={data.ndvi.status} /></td><td data-label="Reference">{data.ndvi.acquiredAt ?? 'No acquisition timestamp'}</td></tr>
            {!data.wallet && <EmptyTableRow colSpan={3}>No project wallet import was included in this dashboard response.</EmptyTableRow>}
          </tbody></table></div>
        </Panel>
      </div>}
    </>
  );
}

export function DashboardPage() {
  return <ProjectGate><DashboardContent /></ProjectGate>;
}
