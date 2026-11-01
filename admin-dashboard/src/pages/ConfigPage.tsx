import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function ConfigPage() { return <ResourceTable title="System config" loader={() => api.getConfig()} />; }

