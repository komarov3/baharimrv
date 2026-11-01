import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function AuditPage() {
  const { data, loading, error } = useApiData(() => api.getAuditEvents());
  if (loading) return <p>Loading audit events...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Audit events</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Time</th><th>User</th><th>Action</th><th>Resource</th><th>Status</th></tr></thead><tbody>
        {data?.items.map((a) => <tr key={a.id}><td>{a.createdAt}</td><td>{a.userId}</td><td>{a.action}</td><td>{a.resource}</td><td>{a.status}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
