import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function MethodologiesPage() { return <ResourceTable title="Methodologies" loader={() => api.getMethodologies()} />; }
