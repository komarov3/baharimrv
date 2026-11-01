import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function ProjectsPage() {
  const { data, loading, error } = useApiData(() => api.getProjects());
  if (loading) return <p>Loading projects...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Projects</h1>
      <div className="table-scroll"><table className="ledger-table"><thead><tr><th>Name</th><th>Community group</th><th>Type</th><th>Created</th></tr></thead><tbody>
        {data?.items.map((p) => <tr key={p.id}><td>{p.name}</td><td>{p.communityGroup ?? ''}</td><td>{p.conservationType ?? ''}</td><td>{p.createdAt}</td></tr>)}
      </tbody></table></div>
    </section>
  );
}
