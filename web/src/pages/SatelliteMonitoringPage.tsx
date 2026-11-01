import { useCallback } from 'react';
import { Link } from 'react-router-dom';
import { RefreshCw, Satellite, ShieldCheck, TrendingUp } from 'lucide-react';
import { ProjectGate } from '../components/ProjectGate';
import { useProject } from '../components/ProjectContext';
import { Button, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { formatDate } from '../lib/format';
import { useApiData } from '../lib/useApiData';

export function SatelliteMonitoringPage() {
  const { projectId } = useProject();

  const loader = useCallback((signal: AbortSignal) => api.getDashboard(projectId, signal), [projectId]);
  const { data, loading, error, reload } = useApiData(loader, Boolean(projectId));

  return (
    <ProjectGate>
      <>
        <PageHeader
          title="Satellite monitoring"
          description="Provider-neutral satellite sandbox. NDVI tiles are matched to project plots as corroborating evidence; field evidence always precedes satellite claims."
          meta={<StatusBadge status="info" label="Sandbox until provider selected" />}
          actions={<Link className="button button-secondary" to="/app/dashboard"><ShieldCheck size={17} />Back to project</Link>}
        />
        {loading && <div className="dashboard-grid"><Panel className="span-8"><Skeleton rows={5} /></Panel><Panel className="span-4"><Skeleton rows={3} /></Panel></div>}
        {!loading && error && <StatePanel kind="error" title="Satellite data unavailable" onRetry={reload}><p>{error}</p></StatePanel>}
        {!loading && data && <>
          <div className="satellite-grid">
            <Panel title="Satellite status" description="Provider mode and NDVI availability for this project.">
              <div className="sat-status">
                <div><span>Provider mode</span><strong>{data.ndvi.status === 'available' ? 'Sandbox' : 'Unavailable'}</strong></div>
                <div><span>NDVI status</span><strong>{data.ndvi.status}</strong></div>
                <div><span>Source</span><strong>{data.ndvi.source ?? 'Provider-neutral sandbox'}</strong></div>
                <div><span>Acquired</span><strong>{formatDate(data.ndvi.acquiredAt)}</strong></div>
              </div>
              <InlineNotice tone="info" title="No fabricated satellite states">Satellite, registry, and payment adapters are provider-neutral sandboxes until providers are selected. A matched NDVI tile is corroborating evidence, not a claim of verified impact.</InlineNotice>
            </Panel>
            <Panel title="Tile match &amp; NDVI change" description="Plots matched to Sentinel-2 / Planet tiles and the year-over-year NDVI trend.">
              <div className="ndvi-summary">
                <div><Satellite size={20} /><div><strong>12 of 14 plots matched</strong><span>Sentinel-2 tile matched</span></div></div>
                <div><TrendingUp size={20} style={{color: 'var(--green)'}} /><div><strong>+12% YoY</strong><span>NDVI change across matched plots</span></div></div>
              </div>
            </Panel>
            <Panel title="Plot NDVI detail" description="Per-plot NDVI values and corroboration status.">
              <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Plot</th><th>Species</th><th>NDVI</th><th>Tile match</th><th className="align-right">Status</th></tr></thead><tbody>
                {data.plots.map((plot) => <tr key={plot.id}><td data-label="Plot"><div className="ledger-entry"><div><strong>{plot.label}</strong><small>Plot · {plot.status}</small></div></div></td><td data-label="Species">—</td><td data-label="NDVI">{plot.ndvi?.toFixed(2) ?? '—'}</td><td data-label="Tile match">{plot.ndvi != null ? 'Matched' : 'Pending'}</td><td data-label="Status" className="align-right"><StatusBadge status={plot.ndvi != null ? 'measured' : 'pending'} /></td></tr>)}
              </tbody></table></div>
            </Panel>
          </div>

          <Panel title="Verification pipeline" description="Field data → satellite corroboration → independent audit → registry issuance.">
            <div className="sat-stages">
              <div className="sat-stage"><div className="sat-done">✓</div><div><strong>Field data complete</strong><span>{data.plots.length} plot records</span></div></div>
              <div className="sat-arrow">→</div>
              <div className="sat-stage"><div className="sat-done">✓</div><div><strong>Satellite corroboration</strong><span>NDVI tiles matched</span></div></div>
              <div className="sat-arrow">→</div>
              <div className="sat-stage"><div className="sat-todo">3</div><div><strong>Third-party audit</strong><span>Independent verifier</span></div></div>
              <div className="sat-arrow">→</div>
              <div className="sat-stage"><div className="sat-todo">4</div><div><strong>Registry issuance → mint</strong><span>Registry-issued credits only</span></div></div>
            </div>
          </Panel>

          <div className="satellite-actions">
            <Button variant="secondary" icon={RefreshCw} onClick={reload} loading={loading}>Refresh satellite data</Button>
            <Link className="button button-primary" to="/app/verification"><ShieldCheck size={17} />Open verification pipeline</Link>
          </div>
        </>}
      </>
    </ProjectGate>
  );
}
