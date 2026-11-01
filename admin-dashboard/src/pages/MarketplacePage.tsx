import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function MarketplacePage() { return <ResourceTable title="Marketplace listings" loader={() => api.getListings()} />; }
