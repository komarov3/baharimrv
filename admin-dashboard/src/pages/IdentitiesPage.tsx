import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function IdentitiesPage() {
  const { data, loading, error } = useApiData(() => api.getIdentities());
  if (loading) return <p>Loading identities...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Identities</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>User</th><th>State</th><th>Network</th><th>Attempts</th></tr></thead><tbody>
        {data?.items.map((i) => <tr key={i.id}><td>{i.userId}</td><td>{i.state}</td><td>{i.network}</td><td>{i.attempts}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
