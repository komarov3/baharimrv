import { ResourceTable } from '../components/ResourceTable';
import { api } from '../lib/api';
export function FinancePage() { return <ResourceTable title="Financial imports" loader={() => api.getFinancialImports()} />; }
