import React from 'react';

let injected = false;
function useSelectStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-select { display: flex; flex-direction: column; font-family: var(--font-sans); }
  .cds-select__label { font-size: 0.75rem; letter-spacing: 0.32px; color: var(--text-secondary); margin-bottom: var(--spacing-03); }
  .cds-select__wrap { position: relative; display: flex; align-items: center; }
  .cds-select__field {
    box-sizing: border-box; width: 100%; height: 40px;
    padding: 0 40px 0 var(--spacing-05); margin: 0;
    background: var(--field-01); color: var(--text-primary);
    border: none; border-bottom: 1px solid var(--border-strong-01); border-radius: 0;
    font-family: var(--font-sans); font-size: 0.875rem; letter-spacing: 0.16px;
    appearance: none; -webkit-appearance: none; cursor: pointer;
    outline: 2px solid transparent; outline-offset: -2px;
    transition: background-color 70ms var(--easing-standard-productive);
  }
  .cds-select__field:hover { background: var(--field-hover-01); }
  .cds-select__field:focus { outline: 2px solid var(--focus); }
  .cds-select__field--sm { height: 32px; }
  .cds-select__field--lg { height: 48px; }
  .cds-select__chev { position: absolute; right: var(--spacing-05); width: 16px; height: 16px; pointer-events: none; color: var(--icon-primary); }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'select');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Select({ label, children, value, defaultValue, size = 'md', disabled = false, onChange, id, className = '', ...rest }) {
  useSelectStyles();
  const inputId = id || `cds-sel-${Math.random().toString(36).slice(2, 8)}`;
  const fieldCls = ['cds-select__field', size === 'sm' ? 'cds-select__field--sm' : size === 'lg' ? 'cds-select__field--lg' : ''].filter(Boolean).join(' ');
  return (
    <div className={['cds-select', className].filter(Boolean).join(' ')}>
      {label ? <label className="cds-select__label" htmlFor={inputId}>{label}</label> : null}
      <div className="cds-select__wrap">
        <select id={inputId} className={fieldCls} value={value} defaultValue={defaultValue} disabled={disabled} onChange={onChange} {...rest}>
          {children}
        </select>
        <svg className="cds-select__chev" viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M8 11L3 6l.7-.7L8 9.6l4.3-4.3.7.7z"/></svg>
      </div>
    </div>
  );
}
