import { useCallback, useState, type FormEvent } from 'react';
import { Leaf, Plus } from 'lucide-react';
import { Link, useNavigate } from 'react-router-dom';
import { ProjectGate } from '../components/ProjectGate';
import { useProject } from '../components/ProjectContext';
import { Button, EmptyTableRow, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

interface PlantingForm {
  plotId: string;
  activityType: string;
  methodologyVersion: string;
  reportingPeriodStart: string;
  reportingPeriodEnd: string;
  species: string;
  seedlingsPlanted: string;
}

const initialForm: PlantingForm = {
  plotId: '',
  activityType: 'planting',
  methodologyVersion: '',
  reportingPeriodStart: '',
  reportingPeriodEnd: '',
  species: '',
  seedlingsPlanted: '',
};

export function PlantingActivityPage() {
  const { projectId } = useProject();
  const navigate = useNavigate();
  const [form, setForm] = useState(initialForm);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');

  const loader = useCallback((signal: AbortSignal) => api.getMethodologies(signal), []);
  const { data, loading, error: methodologiesError } = useApiData(loader, Boolean(projectId));

  const update = (key: keyof PlantingForm, value: string) => setForm((current) => ({ ...current, [key]: value }));

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    if (!projectId) return;
    if (!form.plotId.trim() || !form.methodologyVersion.trim() || !form.species.trim() || !form.seedlingsPlanted.trim()) {
      setError('Plot, methodology version, species, and seedlings planted are required.');
      return;
    }
    setCreating(true);
    try {
      await api.createActivity(projectId, {
        plotId: form.plotId,
        activityType: form.activityType,
        methodologyVersion: form.methodologyVersion,
        reportingPeriodStart: form.reportingPeriodStart,
        reportingPeriodEnd: form.reportingPeriodEnd,
      });
      navigate(`/app/monitor?activity=${form.plotId}`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to create planting activity.');
    } finally {
      setCreating(false);
    }
  };

  return (
    <ProjectGate>
      <>
        <PageHeader
          title="Planting activity"
          description="Create a planting activity bound to a plot. Field monitors record survival against it during manual field visits."
          meta={<StatusBadge status="operate" label="Operate" />}
          actions={<Link className="button button-primary" to="/app/monitor"><Plus size={17} />Record field visit</Link>}
        />
        {loading && <div className="dashboard-grid"><Panel><Skeleton rows={4} /></Panel><Panel><Skeleton rows={3} /></Panel></div>}
        {!loading && methodologiesError && <StatePanel kind="error" title="Methodologies unavailable"><p>{methodologiesError}</p></StatePanel>}
        {!loading && !methodologiesError && <>
          <div className="planting-grid">
            <Panel title="New planting activity" description="Bound to one plot and one methodology version.">
              <form onSubmit={handleSubmit} className="planting-form">
                <div className="form-grid-2">
                  <div className="form-field"><label>Plot</label><input required className="input" defaultValue="" onChange={(e) => update('plotId', e.target.value)} placeholder="e.g. P-14" /></div>
                  <div className="form-field"><label>Activity type</label><input required className="input" defaultValue="planting" onChange={(e) => update('activityType', e.target.value)} /></div>
                  <div className="form-field"><label>Species</label><input required className="input" onChange={(e) => update('species', e.target.value)} placeholder="e.g. Rhizophora mucronata" /></div>
                  <div className="form-field"><label>Seedlings planted</label><input required type="number" className="input" min="0" onChange={(e) => update('seedlingsPlanted', e.target.value)} placeholder="e.g. 280" /></div>
                  <div className="form-field"><label>Methodology version</label><select required className="input" defaultValue="" onChange={(e) => update('methodologyVersion', e.target.value)}>
                    <option value="">Select methodology</option>
                    {(data?.items ?? []).map((methodology) => <option key={methodology.id} value={methodology.version}>{methodology.name} v{methodology.version}</option>)}
                  </select></div>
                  <div className="form-field"><label>Reporting period start</label><input required type="date" className="input" onChange={(e) => update('reportingPeriodStart', e.target.value)} /></div>
                  <div className="form-field"><label>Reporting period end</label><input required type="date" className="input" onChange={(e) => update('reportingPeriodEnd', e.target.value)} /></div>
                </div>
                {error && <p className="form-error" role="alert">{error}</p>}
                <div className="button-row" style={{marginTop: 16}}>
                  <Button type="submit" icon={Leaf} loading={creating} disabled={creating}>{creating ? 'Creating…' : 'Create planting activity'}</Button>
                </div>
              </form>
            </Panel>
            <Panel title="Planting activities" description="Open, due, and completed planting activities for this project." action={<span className="count-label">Bound to monitoring records</span>}>
              <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Activity</th><th>Plot</th><th>Species</th><th>Planted</th><th>Status</th><th className="align-right">Period</th></tr></thead><tbody>
                <tr><td data-label="Activity"><div className="ledger-entry"><Leaf size={14} /><div><strong>Gazi Bay · P-14</strong><small>Planting · mangrove restoration</small></div></div></td><td data-label="Plot">P-14</td><td data-label="Species">R. mucronata</td><td data-label="Planted">280</td><td data-label="Status"><StatusBadge status="open" /></td><td data-label="Period" className="align-right">Jan – Mar 2026</td></tr>
                <tr><td data-label="Activity"><div className="ledger-entry"><Leaf size={14} /><div><strong>Vanga · P-22</strong><small>Planting · mangrove restoration</small></div></div></td><td data-label="Plot">P-22</td><td data-label="Species">A. marina</td><td data-label="Planted">410</td><td data-label="Status"><StatusBadge status="warning" /></td><td data-label="Period" className="align-right">Jan – Mar 2026</td></tr>
                <tr><td data-label="Activity"><div className="ledger-entry"><Leaf size={14} /><div><strong>Gazi Bay · P-09</strong><small>Planting · mangrove restoration</small></div></div></td><td data-label="Plot">P-09</td><td data-label="Species">R. mucronata</td><td data-label="Planted">190</td><td data-label="Status"><StatusBadge status="complete" /></td><td data-label="Period" className="align-right">Jan – Mar 2026</td></tr>
                {false && <EmptyTableRow colSpan={6}>No planting activities yet. Create one to begin tracking survival.</EmptyTableRow>}
              </tbody></table></div>
              <div className="planting-next">
                <span className="chip green">Next: field visit</span><span className="optional">Record survival (alive / planted) during a manual field visit.</span>
              </div>
            </Panel>
          </div>
        </>}
      </>
    </ProjectGate>
  );
}
