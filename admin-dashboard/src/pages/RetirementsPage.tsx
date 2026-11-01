import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function RetirementsPage() { return <ResourceTable title="Retirements" loader={() => api.getRetirements()} />; }
