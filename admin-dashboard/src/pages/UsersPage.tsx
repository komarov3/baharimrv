import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function UsersPage() {
  const { data, loading, error } = useApiData(() => api.getUsers());
  if (loading) return <p>Loading users...</p>;
  if (error) return <p className="error">{error}</p>;
  return (
    <section>
      <h1>Users</h1>
      <div className="table-scroll">
        <table className="ledger-table">
          <thead><tr><th>Name</th><th>Email</th><th>Role</th><th>Status</th><th>Identity</th></tr></thead>
          <tbody>
            {data?.items.map((u) => (
              <tr key={u.id}>
                <td>{u.name}</td><td>{u.email}</td><td>{u.role}</td><td>{u.status}</td><td>{(u as any).identityState ?? 'unknown'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
