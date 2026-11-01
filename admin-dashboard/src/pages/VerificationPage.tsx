import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function VerificationPage() {
  const { data, loading, error } = useApiData(() => api.getVerificationRequests());
  if (loading) return <p>Loading verification requests...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Verification requests</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>ID</th><th>Activity</th><th>Verifier</th><th>Status</th><th>Created</th></tr></thead><tbody>
        {data?.items.map((v) => <tr key={v.id}><td>{v.id}</td><td>{v.activityId}</td><td>{v.verifierId}</td><td>{v.status}</td><td>{v.createdAt}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
