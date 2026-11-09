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
            <Panel title="Satellite status" description="Provider mode and NDVI availability for this project." className="mw-card">
              <div className="mw-inp" style={{flexDirection:'column',gap:4}}>
                <span className="mw-ttl-xs">Provider mode</span><strong>{data.ndvi.status === 'available' ? 'Sandbox' : 'Unavailable'}</strong>
                <span className="mw-ttl-xs">NDVI status</span><strong>{data.ndvi.status}</strong>
                <span className="mw-ttl-xs">Source</span><strong>{data.ndvi.source ?? 'Provider-neutral sandbox'}</strong>
                <span className="mw-ttl-xs">Acquired</span><strong>{formatDate(data.ndvi.acquiredAt)}</strong>
              </div>
              <InlineNotice tone="info" title="No fabricated satellite states">Satellite, registry, and payment adapters are provider-neutral sandboxes until providers are selected. A matched NDVI tile is corroborating evidence, not a claim of verified impact.</InlineNotice>
            </Panel>
            <Panel title="Tile match &amp; NDVI change" description="Plots matched to Sentinel-2 / Planet tiles and the year-over-year NDVI trend." className="mw-card">
              <div className="mw-grid2">
                <div className="mw-card"><div className="mw-icn cyan"><Satellite size={15} /></div><div><strong>12 of 14 plots matched</strong><span className="xs mut">Sentinel-2 tile matched</span></div></div>
                <div className="mw-card"><div className="mw-icn green"><TrendingUp size={15} /></div><div><strong>+12% YoY</strong><span className="xs mut">NDVI change across matched plots</span></div></div>
              </div>
            </Panel>
            <Panel title="Plot NDVI detail" description="Per-plot NDVI values and corroboration status." className="mw-card">
              <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Plot</th><th>Species</th><th>NDVI</th><th>Tile match</th><th className="align-right">Status</th></tr></thead><tbody>
                {data.plots.map((plot) => <tr key={plot.id}><td data-label="Plot"><div className="ledger-entry"><div><strong>{plot.label}</strong><small>Plot · {plot.status}</small></div></div></td><td data-label="Species">—</td><td data-label="NDVI">{plot.ndvi?.toFixed(2) ?? '—'}</td><td data-label="Tile match">{plot.ndvi != null ? 'Matched' : 'Pending'}</td><td data-label="Status" className="align-right"><StatusBadge status={plot.ndvi != null ? 'measured' : 'pending'} /></td></tr>)}
              </tbody></table></div>
            </Panel>
          </div>

          <Panel title="Verification pipeline" description="Field data → satellite corroboration → independent audit → registry issuance." className="mw-card">
            <div className="mw-step">
              <div className="mw-st-ic mw-st-ok"><ShieldCheck size={14} /></div><div className="mw-grow"><div className="b">Field data complete</div><div className="xs mut">{data.plots.length} plot records</div></div>
            </div>
            <div className="mw-st-line"></div>
            <div className="mw-step">
              <div className="mw-st-ic mw-st-ok"><Satellite size={14} /></div><div className="mw-grow"><div className="b">Satellite corroboration</div><div className="xs mut">NDVI tiles matched</div></div>
            </div>
            <div className="mw-st-line"></div>
            <div className="mw-step">
              <div className="mw-st-ic mw-st-todo">3</div><div className="mw-grow"><div className="b">Third-party audit</div><div className="xs mut">Independent verifier</div></div>
            </div>
            <div className="mw-st-line"></div>
            <div className="mw-step">
              <div className="mw-st-ic mw-st-todo">4</div><div className="mw-grow"><div className="b">Registry issuance → mint</div><div className="xs mut">Registry-issued credits only</div></div>
            </div>
          </Panel>

          <div className="satellite-actions">
            <Button variant="secondary" icon={RefreshCw} onClick={reload} loading={loading} className="mw-btn">Refresh satellite data</Button>
            <Link className="mw-btn mw-btn--g" to="/app/verification"><ShieldCheck size={17} />Open verification pipeline</Link>
          </div>
        </>}
      </>
    </ProjectGate>
  );
}
