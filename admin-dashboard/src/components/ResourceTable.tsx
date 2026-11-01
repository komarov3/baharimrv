import { useApiData } from '../lib/useApiData';

export function ResourceTable({ title, loader }: { title: string; loader: () => Promise<{ items: any[] }> }) {
  const { data, loading, error } = useApiData(loader);
  if (loading) return <p>Loading {title.toLowerCase()}...</p>;
  if (error) return <p className="error">{error}</p>;
  const items = data?.items ?? [];
  const columns = items.length ? Object.keys(items[0]) : [];
  return (
    <section>
      <h1>{title}</h1>
      {items.length === 0 ? <p>No records.</p> : (
        <div className="table-scroll">
          <table className="ledger-table">
            <thead><tr>{columns.map((c) => <th key={c}>{c}</th>)}</tr></thead>
            <tbody>
              {items.map((row, idx) => (
                <tr key={row.id ?? idx}>
                  {columns.map((c) => <td key={c}>{String(row[c] ?? '')}</td>)}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
