import React from 'react';

let injected = false;
function useCheckboxStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-checkbox { display: inline-flex; align-items: flex-start; gap: var(--spacing-03); font-family: var(--font-sans); cursor: pointer; user-select: none; }
  .cds-checkbox input { position: absolute; opacity: 0; width: 0; height: 0; }
  .cds-checkbox__box { position: relative; flex: none; width: 16px; height: 16px; margin-top: 1px; border: 1px solid var(--icon-primary); border-radius: 2px; background: transparent; transition: background-color 70ms, border-color 70ms; }
  .cds-checkbox__box svg { position: absolute; inset: 0; width: 16px; height: 16px; fill: none; stroke: var(--icon-inverse); stroke-width: 2; opacity: 0; }
  .cds-checkbox input:checked + .cds-checkbox__box { background: var(--icon-primary); border-color: var(--icon-primary); }
  .cds-checkbox input:checked + .cds-checkbox__box svg { opacity: 1; }
  .cds-checkbox input:focus-visible + .cds-checkbox__box { outline: 2px solid var(--focus); outline-offset: 1px; }
  .cds-checkbox__label { font-size: 0.875rem; line-height: 1.28572; letter-spacing: 0.16px; color: var(--text-primary); }
  .cds-checkbox--disabled { cursor: not-allowed; }
  .cds-checkbox--disabled .cds-checkbox__box { border-color: var(--border-disabled); }
  .cds-checkbox--disabled .cds-checkbox__label { color: var(--text-disabled); }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'checkbox');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Checkbox({ label, checked, defaultChecked, disabled = false, onChange, id, className = '', ...rest }) {
  useCheckboxStyles();
  const inputId = id || `cds-cb-${Math.random().toString(36).slice(2, 8)}`;
  return (
    <label className={['cds-checkbox', disabled ? 'cds-checkbox--disabled' : '', className].filter(Boolean).join(' ')} htmlFor={inputId}>
      <input id={inputId} type="checkbox" checked={checked} defaultChecked={defaultChecked} disabled={disabled} onChange={onChange} {...rest} />
      <span className="cds-checkbox__box" aria-hidden="true">
        <svg viewBox="0 0 16 16"><polyline points="3.5,8.5 6.5,11.5 12.5,4.5" /></svg>
      </span>
      {label ? <span className="cds-checkbox__label">{label}</span> : null}
    </label>
  );
}
