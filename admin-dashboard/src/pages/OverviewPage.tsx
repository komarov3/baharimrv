import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function OverviewPage() {
  const { data, loading, error } = useApiData(() => api.getOverview());
  if (loading) return <p>Loading overview...</p>;
  if (error) return <p className="error">{error}</p>;
  if (!data) return null;
  const entries = Object.entries(data);
  return (
    <section>
      <h1>Platform overview</h1>
      <div className="metric-grid">
        {entries.map(([key, value]) => (
          <div key={key} className="card metric-card">
            <span>{key}</span>
            <strong>{String(value)}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}
