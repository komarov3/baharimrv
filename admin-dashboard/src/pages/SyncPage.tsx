import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function SyncPage() { return <ResourceTable title="Sync actions" loader={() => api.getSyncActions()} />; }
