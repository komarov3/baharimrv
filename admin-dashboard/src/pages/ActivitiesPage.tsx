import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function ActivitiesPage() {
  const { data, loading, error } = useApiData(() => api.getActivities());
  if (loading) return <p>Loading activities...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Activities</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>ID</th><th>Project</th><th>Status</th><th>Created</th></tr></thead><tbody>
        {data?.items.map((a) => <tr key={a.id}><td>{a.id}</td><td>{a.projectId}</td><td>{a.status}</td><td>{a.createdAt}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
