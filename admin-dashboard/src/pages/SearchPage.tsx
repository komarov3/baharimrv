import { useState } from 'react';
import { api } from '../lib/api';

export function SearchPage() {
  const [q, setQ] = useState('');
  const [results, setResults] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  async function run() {
    setLoading(true);
    try { setResults(await api.search(q)); } finally { setLoading(false); }
  }
  return (
    <section>
      <h1>Global search</h1>
      <div className="field-row">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search users, projects, activities, proofs" />
        <button className="button button-primary" onClick={run}>Search</button>
      </div>
      {loading && <p>Searching...</p>}
      {results && <pre>{JSON.stringify(results, null, 2)}</pre>}
    </section>
  );
}
