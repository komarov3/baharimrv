import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function AnchorsPage() {
  const { data, loading, error } = useApiData(() => api.getAnchors());
  if (loading) return <p>Loading anchors...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Anchor attempts</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>ID</th><th>Status</th><th>Commitment</th><th>Transaction</th><th>Created</th></tr></thead><tbody>
        {data?.items.map((a) => <tr key={a.id}><td>{a.id}</td><td>{a.status}</td><td>{a.commitmentHash ?? ''}</td><td>{a.transactionReference ?? ''}</td><td>{a.createdAt}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
