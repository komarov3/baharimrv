import { AlertCircle, CheckCircle2, Inbox, LoaderCircle, RefreshCw, WifiOff, type LucideIcon } from 'lucide-react';
import { forwardRef, type ButtonHTMLAttributes, type HTMLAttributes, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { evidenceLabel, statusTone } from '../lib/format';

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <Link to="/app/dashboard" className="brand" aria-label="BahariMRV home">
      <span className="brand-mark" aria-hidden="true"><span /><span /><span /></span>
      {!compact && <span className="brand-type"><strong>BahariMRV</strong><small>BahariLink conservation</small></span>}
    </Link>
  );
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'quiet' | 'danger';
  loading?: boolean;
  icon?: LucideIcon;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button({ variant = 'primary', loading, icon: Icon, children, disabled, className = '', ...props }, ref) {
  return (
    <button ref={ref} className={`button button-${variant} ${className}`} disabled={disabled || loading} aria-busy={loading || undefined} {...props}>
      {loading ? <LoaderCircle className="spin" size={17} aria-hidden="true" /> : Icon ? <Icon size={17} aria-hidden="true" /> : null}
      <span>{children}</span>
    </button>
  );
});

export function StatusBadge({ status, label }: { status: string; label?: string }) {
  return <span className={`status status-${statusTone(status)}`}><span className="status-dot" aria-hidden="true" />{label ?? evidenceLabel(status)}</span>;
}

export function PageHeader({ title, description, actions, meta }: { title: string; description: string; actions?: ReactNode; meta?: ReactNode }) {
  return (
    <header className="page-header">
      <div className="page-heading">
        <div className="page-title-row"><h1>{title}</h1>{meta}</div>
        <p>{description}</p>
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  );
}

export function Panel({ title, description, action, children, className = '', ...props }: HTMLAttributes<HTMLElement> & { title?: string; description?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className={`panel ${className}`} {...props}>
      {(title || action) && <div className="panel-header"><div>{title && <h2>{title}</h2>}{description && <p>{description}</p>}</div>{action}</div>}
      {children}
    </section>
  );
}

export function Skeleton({ rows = 4, label = 'Loading current data' }: { rows?: number; label?: string }) {
  return (
    <div className="skeleton-stack" role="status" aria-label={label}>
      <div className="skeleton skeleton-heading" />
      {Array.from({ length: rows }, (_, index) => <div key={index} className={`skeleton skeleton-row skeleton-row-${index % 3}`} />)}
    </div>
  );
}

export function StatePanel({ kind, title, children, onRetry, action }: { kind: 'empty' | 'error' | 'offline' | 'success'; title: string; children: ReactNode; onRetry?: () => void; action?: ReactNode }) {
  const Icon = kind === 'error' ? AlertCircle : kind === 'offline' ? WifiOff : kind === 'success' ? CheckCircle2 : Inbox;
  return (
    <div className={`state-panel state-${kind}`} role={kind === 'error' ? 'alert' : 'status'}>
      <Icon size={24} aria-hidden="true" />
      <div><h3>{title}</h3><div className="state-copy">{children}</div>
        {(onRetry || action) && <div className="state-actions">{onRetry && <Button variant="secondary" icon={RefreshCw} onClick={onRetry}>Try again</Button>}{action}</div>}
      </div>
    </div>
  );
}

export function InlineNotice({ tone = 'info', title, children }: { tone?: 'info' | 'warning' | 'danger' | 'success'; title: string; children: ReactNode }) {
  return <div className={`notice notice-${tone}`} role={tone === 'danger' ? 'alert' : 'status'}><AlertCircle size={18} aria-hidden="true" /><div><strong>{title}</strong><p>{children}</p></div></div>;
}

export function FormError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="form-error" role="alert"><AlertCircle size={15} aria-hidden="true" />{message}</p>;
}

export function DataPair({ label, value, mono = false }: { label: string; value: ReactNode; mono?: boolean }) {
  return <div className="data-pair"><dt>{label}</dt><dd className={mono ? 'mono' : undefined}>{value}</dd></div>;
}

export function EmptyTableRow({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return <tr><td colSpan={colSpan}><div className="table-empty"><Inbox size={20} aria-hidden="true" />{children}</div></td></tr>;
}
