import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function IntegrationsPage() {
  const { data, loading, error } = useApiData(() => api.getIntegrations());
  if (loading) return <p>Loading integrations...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Integrations</h1>
      <pre>{JSON.stringify(data, null, 2)}</pre>
    </section>
  );
}
