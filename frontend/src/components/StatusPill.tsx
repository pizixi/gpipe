import React from 'react';

export type StatusVariant = 'online' | 'offline' | 'enabled' | 'disabled' | 'running' | 'failed' | 'waiting' | 'starting' | 'unverified';

interface Props {
  label: string;
  variant: StatusVariant;
}

const StatusPill: React.FC<Props> = ({ label, variant }) => {
  const icon = (() => {
    switch (variant) {
      case 'online':
        return (
          <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d="M3 8.2a5 5 0 0 1 10 0M5.3 10.3a2.7 2.7 0 0 1 5.4 0" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
            <circle cx="8" cy="12.2" r="1.2" fill="currentColor" />
          </svg>
        );
      case 'offline':
        return (
          <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d="M3 8.2a5 5 0 0 1 10 0M5.3 10.3a2.7 2.7 0 0 1 5.4 0" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
            <path d="M3.3 3.3 12.7 12.7" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
          </svg>
        );
      case 'enabled':
      case 'running':
        return (
          <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <circle cx="8" cy="8" r="5.6" stroke="currentColor" strokeWidth="1.3" />
            <path d="M5.4 8.1 7.1 9.8 10.7 6.2" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        );
      case 'disabled':
      case 'failed':
        return (
          <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <circle cx="8" cy="8" r="5.6" stroke="currentColor" strokeWidth="1.3" />
            <path d="M6 6h4v4H6z" fill="currentColor" />
          </svg>
        );
      case 'waiting':
      case 'starting':
      case 'unverified':
        return (
          <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <circle cx="8" cy="8" r="5.6" stroke="currentColor" strokeWidth="1.3" />
            <path d="M8 4.8v3.6l2.3 1.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        );
      default:
        return null;
    }
  })();

  return (
    <span className={`status-pill status-pill--${variant}`}>
      <span className="status-pill-icon">{icon}</span>
      <span className="status-pill-label">{label}</span>
    </span>
  );
};

export default StatusPill;
