import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function ExportsPage() { return <ResourceTable title="Export jobs" loader={() => api.getExports()} />; }
