import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function ProvenancePage() { return <ResourceTable title="Provenance events" loader={() => api.getProvenanceEvents()} />; }
