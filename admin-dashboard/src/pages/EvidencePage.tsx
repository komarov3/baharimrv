import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function EvidencePage() { return <ResourceTable title="Evidence" loader={() => api.getEvidence()} />; }
