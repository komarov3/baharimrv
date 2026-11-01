import { Anchor, ArrowUpRight, Check, FileInput, GitBranch, RefreshCcw, ShieldCheck, WalletCards } from 'lucide-react';
import type { ChainNetwork, ProvenanceEvent } from '../types/api';
import { formatDate, networkName } from '../lib/format';
import { StatePanel, StatusBadge } from './ui';

const eventPresentation = {
  evidence_submitted: { label: 'Evidence submitted', icon: FileInput, explanation: 'Evidence existence only · not a completion or achievement claim' },
  achieved_unverified: { label: 'Achievement completed', icon: Check, explanation: 'Manager attested · achieved_unverified · independent validity not established' },
  achievement_verified: { label: 'Achievement verified', icon: ShieldCheck, explanation: 'Independently approved conservation outcome' },
  custody_changed: { label: 'Custody changed', icon: WalletCards, explanation: 'Registry-backed ownership or custody event' },
  retired: { label: 'Credits retired', icon: Anchor, explanation: 'Retirement provenance following registry confirmation' },
  correction: { label: 'Additive correction', icon: GitBranch, explanation: 'Superseding record · original history preserved' },
};

function referenceLabel(network?: ChainNetwork) {
  return network ? `${networkName(network)} reference` : 'Ledger reference';
}

export function ProvenanceTimeline({ events }: { events: ProvenanceEvent[] }) {
  if (!events.length) return <StatePanel kind="empty" title="No provenance events yet"><p>Evidence submission, manager completion, and independent verification will appear as separate chronological events.</p></StatePanel>;
  return (
    <ol className="provenance-timeline">
      {events.map((event) => {
        const presentation = eventPresentation[event.type] ?? { label: event.type, icon: RefreshCcw, explanation: event.detail };
        const Icon = presentation.icon;
        return <li key={event.id} className={`provenance-event event-${event.type}`}>
          <div className="timeline-marker"><Icon size={17} aria-hidden="true" /></div>
          <div className="timeline-content">
            <div className="timeline-title"><div><strong>{presentation.label}</strong><span>{presentation.explanation}</span></div><StatusBadge status={event.status} /></div>
            <p>{event.detail}</p>
            <dl><div><dt>Actor</dt><dd>{event.actor}</dd></div><div><dt>Recorded</dt><dd>{formatDate(event.timestamp)}</dd></div>{event.commitment && <div><dt>Commitment</dt><dd className="mono hash-value">{event.commitment}</dd></div>}{event.chainReference && <div><dt>{referenceLabel(event.network)}</dt><dd className="mono hash-value">{event.chainReference}</dd></div>}</dl>
            {event.explorerUrl && <a className="text-link" href={event.explorerUrl} target="_blank" rel="noreferrer">Open configured explorer <ArrowUpRight size={14} /></a>}
          </div>
        </li>;
      })}
    </ol>
  );
}
