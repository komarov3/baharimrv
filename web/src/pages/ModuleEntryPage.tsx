import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Leaf, ShieldCheck, UserRound } from 'lucide-react';
import { Button, PageHeader, Panel, StatePanel } from '../components/ui';
import { api } from '../lib/api';
import { useApiData } from '../lib/useApiData';

export function ModuleEntryPage() {
  const navigate = useNavigate();

  const loader = useCallback((signal: AbortSignal) => api.getIdentity(signal), []);
  const { data, loading, error, reload } = useApiData(loader);

  return (
    <>
      <PageHeader
        title="Module entry"
        description="Enter the Mikoko conservation module. Sign in with your shared Blue ID — the same account used across Nanga and Jahazi."
        meta={data ? <span style={{fontSize:'11px',color:'var(--muted)'}}>Blue ID identity · {data.network === 'iota_testnet' ? 'IOTA testnet' : 'Sandbox ledger'}</span> : undefined}
      />
      {loading && <Panel><StatePanel kind="empty" title="Loading identity…"><p>Checking your Blue ID session.</p></StatePanel></Panel>}
      {!loading && error && <StatePanel kind="error" title="Identity unavailable" onRetry={reload}><p>{error}</p></StatePanel>}
      {!loading && !error && <>
        <div className="m0-splash">
          <div className="m0-mark">
            <Leaf size={34} />
          </div>
          <div className="m0-title">Mikoko</div>
          <div className="m0-sub">Grow the coast. Verify the carbon. Share the value.</div>
          <div className="m0-divider">
            <Button variant="primary" icon={ShieldCheck} onClick={() => navigate('/app/dashboard')}>
              Sign in with Blue ID <span style={{fontSize:'9px'}}>· same account as Nanga &amp; Jahazi</span>
            </Button>
            <Button variant="secondary" icon={UserRound} onClick={() => navigate('/app/profile')}>
              Join as a community monitor
            </Button>
            <div className="mw-row" style={{justifyContent:'center',gap:6,color:'#a7e8cd'}}>
              <ShieldCheck size={13} style={{color:'#7de3f5'}} /><span style={{fontSize:'9px'}}>Shared Blue ID across modules</span>
            </div>
          </div>
        </div>
        <div style={{marginTop:20,display:'flex',flexDirection:'column',gap:8}}>
          <Panel title="What Mikoko does" description="Conservation module of BahariLink — mangrove restoration, field MRV, satellite corroboration, verified credits.">
            <div className="mw-grid2">
              <div className="mw-card"><div className="mw-icn green"><Leaf size={15} /></div><div><strong>Field MRV</strong><span className="xs mut">GPS, survival, photos, voice · offline-first</span></div></div>
              <div className="mw-card"><div className="mw-icn cyan"><svg width="16" height="16" viewBox="0 0 24 24"><rect x="9" y="9" width="6" height="6" rx="1" transform="rotate(45 12 12)"/></svg></div><div><strong>Satellite NDVI</strong><span className="xs mut">Sentinel-2 / Planet corroboration</span></div></div>
              <div className="mw-card"><div className="mw-icn violet"><ShieldCheck size={15} /></div><div><strong>Verification</strong><span className="xs mut">Independent audit → registry → mint</span></div></div>
              <div className="mw-card"><div className="mw-icn amber"><ShieldCheck size={15} /></div><div><strong>Carbon marketplace</strong><span className="xs mut">Buy, retire, verify credits</span></div></div>
            </div>
          </Panel>
          <Panel title="Shared identity" description="One Blue ID, one wallet, one profile across all BahariLink modules.">
            <div className="mw-card" style={{gap:4}}>
              <div className="mw-row"><span className="xs mut">Network</span><span className="xs">{data?.network === 'iota_testnet' ? 'IOTA testnet' : 'Sandbox ledger'}</span></div>
              <div className="mw-row"><span className="xs mut">Status</span><span className={`xs ${data?.status === 'active' ? 'green' : 'mut'}`}>{data?.status ?? 'pending'}</span></div>
              <div className="mw-row"><span className="xs mut">DID</span><span className="xs mut">{data?.did?.slice(0, 20) ?? '—'}…</span></div>
            </div>
          </Panel>
        </div>
      </>}
    </>
  );
}
