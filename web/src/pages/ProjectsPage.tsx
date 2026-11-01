import { useCallback, useEffect, useState } from 'react';
import { Plus } from 'lucide-react';
import { Button, FormError, PageHeader, Panel, StatePanel } from '../components/ui';
import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';
import type { Methodology, ProjectPlanner } from '../types/api';

export function ProjectsPage() {
  const projectsLoader = useCallback((signal: AbortSignal) => api.getProjects(signal), []);
  const { data: table, loading, error, reload } = useApiData(projectsLoader);
  const methodologiesLoader = useCallback((signal: AbortSignal) => api.getMethodologies(signal), []);
  const { data: methodologyTable } = useApiData(methodologiesLoader);

  const [showCreate, setShowCreate] = useState(false);
  const [conservationType, setConservationType] = useState('mangrove');
  const [name, setName] = useState('');
  const [communityGroup, setCommunityGroup] = useState('');
  const [species, setSpecies] = useState('');
  const [communityShare, setCommunityShare] = useState('6000');
  const [sites, setSites] = useState('');
  const [methodologyId, setMethodologyId] = useState('');
  const [plotCode, setPlotCode] = useState('P1');
  const [plotSite, setPlotSite] = useState('');
  const [area, setArea] = useState('1.0');
  const [latitude, setLatitude] = useState('-4.05');
  const [longitude, setLongitude] = useState('39.67');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [createdId, setCreatedId] = useState('');

  const methodologies: Methodology[] = methodologyTable?.items ?? [];
  const speciesOptions: Record<string, string[]> = {
    mangrove: ['Rhizophora mucronata', 'Avicennia marina', 'Sonneratia alba', 'Bruguiera gymnorrhiza'],
    seagrass: ['Thalassia hemprichii', 'Cymodocea rotundata', 'Enhalus acoroides', 'Halodule uninervis'],
    reef: ['Acropora spp.', 'Porites spp.', 'Pocillopora spp.', 'Mixed hard coral'],
  };
  const filteredMethodologies = methodologies.filter((methodology) => {
    const text = `${methodology.name} ${methodology.code}`.toLowerCase();
    return text.includes(conservationType) || text.includes(conservationType === 'reef' ? 'coral' : conservationType);
  });
  const availableMethodologies = filteredMethodologies.length ? filteredMethodologies : methodologies;

  useEffect(() => {
    const options = speciesOptions[conservationType] ?? [];
    if (options.length && !options.includes(species)) setSpecies(options[0]);
  }, [conservationType]);

  useEffect(() => {
    if (availableMethodologies.length && !availableMethodologies.some((item) => item.id === methodologyId)) {
      setMethodologyId(availableMethodologies[0].id);
    }
  }, [availableMethodologies, methodologyId]);
  if (!methodologyId && methodologies.length) setMethodologyId(methodologies[0].id);

  const create = async () => {
    setCreating(true);
    setCreateError('');
    setCreatedId('');
    try {
      const siteNames = sites.split(',').map((value) => value.trim()).filter(Boolean);
      if (siteNames.length === 0 && plotSite.trim()) siteNames.push(plotSite.trim());
      if (siteNames.length === 0) siteNames.push('Site 1');
      const input: ProjectPlanner = {
        name: name.trim(),
        methodologyId: methodologyId.trim(),
        conservationType: conservationType,
        communityGroup: communityGroup.trim(),
        species: species.trim(),
        communityShareBasisPoints: Number(communityShare) || 0,
        sites: siteNames,
        plots: [{
          code: plotCode.trim() || 'P1',
          siteName: plotSite.trim() || siteNames[0],
          latitude: Number(latitude) || 0,
          longitude: Number(longitude) || 0,
          areaHectares: area.trim() || '1.0',
        }],
      };
      const result = await api.createProject(input);
      setCreatedId(result.id);
      setShowCreate(false);
      reload();
    } catch (cause) {
      setCreateError(cause instanceof Error ? cause.message : 'The project could not be created.');
    } finally {
      setCreating(false);
    }
  };

  return (
    <>
      <PageHeader title="Projects" description="Create and open conservation projects. Managers can start a community project here; the backend enforces who may create one." actions={<Button variant="secondary" icon={Plus} onClick={() => setShowCreate((value) => !value)}>{showCreate ? 'Cancel' : 'Create project'}</Button>} />
      {loading && <Panel><p>Loading projects…</p></Panel>}
      {error && <StatePanel kind="error" title="Project list unavailable" onRetry={reload}><p>{error}</p></StatePanel>}
      {createdId && <StatePanel kind="success" title="Project created"><p>{createdId}</p></StatePanel>}
      {showCreate && (
        <Panel title="Create a conservation project" description="The first plot and site are created with the project. Methodology selection is required by the backend.">
          <div className="field"><label htmlFor="project-name">Project name</label><input id="project-name" value={name} onChange={(event) => setName(event.target.value)} required /></div>
          <div className="field"><label htmlFor="community-group">Community group</label><input id="community-group" value={communityGroup} onChange={(event) => setCommunityGroup(event.target.value)} /></div>
          <div className="field"><label htmlFor="conservation-type">Type of conservation</label><select id="conservation-type" value={conservationType} onChange={(event) => setConservationType(event.target.value)}><option value="mangrove">Mangrove</option><option value="seagrass">Seagrass</option><option value="reef">Coral reef</option></select></div>
          <div className="field"><label htmlFor="species">Species</label><select id="species" value={species} onChange={(event) => setSpecies(event.target.value)}>{(speciesOptions[conservationType] ?? []).map((option) => <option key={option} value={option}>{option}</option>)}</select></div>
          <div className="field"><label htmlFor="community-share">Community share (basis points)</label><input id="community-share" value={communityShare} onChange={(event) => setCommunityShare(event.target.value)} /></div>
          <div className="field"><label htmlFor="sites">Sites (comma separated)</label><input id="sites" value={sites} onChange={(event) => setSites(event.target.value)} placeholder="e.g. Mikoko Park, Mtwapa Creek" /></div>
          <div className="field"><label htmlFor="methodology">Methodology</label><select id="methodology" value={methodologyId} onChange={(event) => setMethodologyId(event.target.value)}>{availableMethodologies.map((methodology) => <option key={methodology.id} value={methodology.id}>{methodology.name} {methodology.version}</option>)}</select></div>
          <div className="field"><label htmlFor="plot-code">First plot code</label><input id="plot-code" value={plotCode} onChange={(event) => setPlotCode(event.target.value)} /></div>
          <div className="field"><label htmlFor="plot-site">First plot site name</label><input id="plot-site" value={plotSite} onChange={(event) => setPlotSite(event.target.value)} /></div>
          <div className="field"><label htmlFor="area">Area (ha)</label><input id="area" value={area} onChange={(event) => setArea(event.target.value)} /></div>
          <div className="field"><label htmlFor="latitude">Latitude</label><input id="latitude" value={latitude} onChange={(event) => setLatitude(event.target.value)} /></div>
          <div className="field"><label htmlFor="longitude">Longitude</label><input id="longitude" value={longitude} onChange={(event) => setLongitude(event.target.value)} /></div>
          {methodologies.length === 0 && <StatePanel kind="empty" title="No methodologies registered"><p>A BahariMRV administrator must register a methodology before a project can be created.</p></StatePanel>}
          <FormError message={createError} />
          <Button loading={creating} onClick={create} disabled={!name.trim() || !methodologyId.trim() || methodologies.length === 0}>Create project</Button>
        </Panel>
      )}
      {!loading && table && table.rows.length === 0 && (
        <StatePanel kind="empty" title="No projects yet">
          <p>Create your first conservation project to begin monitoring. Managers can use the button above.</p>
        </StatePanel>
      )}
      {!loading && table && table.rows.length > 0 && (
        <Panel title="Authorized projects">
          <ul>{table.rows.map((row) => <li key={row.id}>{row.name} · {row.communityGroup || 'Community group not recorded'}</li>)}</ul>
        </Panel>
      )}
    </>
  );
}
