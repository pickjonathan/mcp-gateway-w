import React from 'react';

let injected = false;
function useProgressStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-progress { display: flex; flex-direction: column; gap: var(--spacing-03); width: 100%; font-family: var(--font-sans); }
  .cds-progress__top { display: flex; justify-content: space-between; align-items: baseline; }
  .cds-progress__label { font-size: 0.875rem; line-height: 1.28572; letter-spacing: 0.16px; color: var(--text-primary); }
  .cds-progress__value { font-size: 0.75rem; color: var(--text-secondary); font-variant-numeric: tabular-nums; }
  .cds-progress__track { position: relative; width: 100%; height: 8px; background: var(--layer-accent-01); overflow: hidden; }
  .cds-progress__bar { height: 100%; background: var(--interactive); transition: width 240ms var(--easing-standard-productive); }
  .cds-progress--indeterminate .cds-progress__bar { width: 30% !important; animation: cds-prog 1.2s var(--easing-standard-productive) infinite; }
  @keyframes cds-prog { 0% { transform: translateX(-100%); } 100% { transform: translateX(400%); } }
  .cds-progress--success .cds-progress__bar { background: var(--support-success); }
  .cds-progress--error .cds-progress__track { box-shadow: inset 0 0 0 1px var(--support-error); }
  .cds-progress--error .cds-progress__bar { background: var(--support-error); }
  .cds-progress__helper { font-size: 0.75rem; color: var(--text-helper); letter-spacing: 0.32px; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'progress');
  el.textContent = css;
  document.head.appendChild(el);
}

export function ProgressBar({ label, value = 0, max = 100, helperText, status = 'active', hideLabel = false, indeterminate = false, className = '', ...rest }) {
  useProgressStyles();
  const pct = Math.max(0, Math.min(100, (value / max) * 100));
  const cls = ['cds-progress', indeterminate ? 'cds-progress--indeterminate' : '', status === 'success' ? 'cds-progress--success' : '', status === 'error' ? 'cds-progress--error' : '', className].filter(Boolean).join(' ');
  return (
    <div className={cls} {...rest}>
      {!hideLabel && (label || !indeterminate) ? (
        <div className="cds-progress__top">
          {label ? <span className="cds-progress__label">{label}</span> : <span />}
          {!indeterminate ? <span className="cds-progress__value">{Math.round(pct)}%</span> : null}
        </div>
      ) : null}
      <div className="cds-progress__track"><div className="cds-progress__bar" style={{ width: pct + '%' }} /></div>
      {helperText ? <span className="cds-progress__helper">{helperText}</span> : null}
    </div>
  );
}
