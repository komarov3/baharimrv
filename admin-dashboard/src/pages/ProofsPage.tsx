import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function ProofsPage() {
  const { data, loading, error } = useApiData(() => api.getProofs());
  if (loading) return <p>Loading proofs...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Proof documents</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>ID</th><th>Activity</th><th>Type</th><th>Status</th><th>Version</th><th>Created</th></tr></thead><tbody>
        {data?.items.map((p) => <tr key={p.id}><td>{p.id}</td><td>{p.activityId}</td><td>{p.proofType}</td><td>{p.status}</td><td>{p.version}</td><td>{p.createdAt}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
