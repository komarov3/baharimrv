import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function CreditBatchesPage() { return <ResourceTable title="Credit batches" loader={() => api.getCreditBatches()} />; }
