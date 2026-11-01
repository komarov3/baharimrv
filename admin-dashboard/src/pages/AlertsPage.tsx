import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function AlertsPage() { return <ResourceTable title="Alerts" loader={() => api.getAlerts()} />; }
