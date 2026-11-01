import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function CorrectionsPage() { return <ResourceTable title="Corrections" loader={() => api.getCorrections()} />; }
