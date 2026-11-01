import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function SatellitePage() { return <ResourceTable title="Satellite passes" loader={() => api.getSatellitePasses()} />; }
