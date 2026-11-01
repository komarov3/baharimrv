import { useApiData } from '../lib/useApiData';
import { api } from '../lib/api';
export function DataQualityPage() {
  const { data, loading, error } = useApiData(() => api.getDataQuality());
  if (loading) return <p>Loading data quality...</p>;
  if (error) return <p className="error">{error}</p>;
  return <section><h1>Data quality</h1><pre>{JSON.stringify(data, null, 2)}</pre></section>;
}
