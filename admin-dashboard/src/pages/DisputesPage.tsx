import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function DisputesPage() { return <ResourceTable title="Disputes" loader={() => api.getDisputes()} />; }
