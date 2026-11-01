import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function PurchasesPage() { return <ResourceTable title="Purchases" loader={() => api.getPurchases()} />; }
