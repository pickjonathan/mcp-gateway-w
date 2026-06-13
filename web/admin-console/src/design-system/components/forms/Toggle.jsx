import React from 'react';

let injected = false;
function useToggleStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-toggle { display: inline-flex; flex-direction: column; gap: var(--spacing-03); font-family: var(--font-sans); }
  .cds-toggle__label { font-size: 0.75rem; letter-spacing: 0.32px; color: var(--text-secondary); }
  .cds-toggle__row { display: inline-flex; align-items: center; gap: var(--spacing-03); cursor: pointer; }
  .cds-toggle input { position: absolute; opacity: 0; width: 0; height: 0; }
  .cds-toggle__track { position: relative; flex: none; width: 48px; height: 24px; background: var(--toggle-off); border-radius: 999px; transition: background-color 110ms var(--easing-standard-productive); }
  .cds-toggle__knob { position: absolute; top: 3px; left: 3px; width: 18px; height: 18px; background: var(--icon-on-color); border-radius: 50%; transition: transform 110ms var(--easing-standard-productive); }
  .cds-toggle input:checked + .cds-toggle__track { background: var(--support-success); }
  .cds-toggle input:checked + .cds-toggle__track .cds-toggle__knob { transform: translateX(24px); }
  .cds-toggle input:focus-visible + .cds-toggle__track { outline: 2px solid var(--focus); outline-offset: 1px; }
  .cds-toggle__state { font-size: 0.875rem; color: var(--text-primary); }
  .cds-toggle--sm .cds-toggle__track { width: 32px; height: 16px; }
  .cds-toggle--sm .cds-toggle__knob { width: 10px; height: 10px; }
  .cds-toggle--sm input:checked + .cds-toggle__track .cds-toggle__knob { transform: translateX(16px); }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'toggle');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Toggle({ label, checked, defaultChecked, size = 'md', labelOn = 'On', labelOff = 'Off', hideStateLabel = false, disabled = false, onChange, id, className = '', ...rest }) {
  useToggleStyles();
  const inputId = id || `cds-tg-${Math.random().toString(36).slice(2, 8)}`;
  const [internal, setInternal] = React.useState(defaultChecked || false);
  const isControlled = checked !== undefined;
  const on = isControlled ? checked : internal;
  return (
    <span className={['cds-toggle', size === 'sm' ? 'cds-toggle--sm' : '', className].filter(Boolean).join(' ')}>
      {label ? <span className="cds-toggle__label">{label}</span> : null}
      <label className="cds-toggle__row" htmlFor={inputId}>
        <input id={inputId} type="checkbox" checked={on} disabled={disabled}
          onChange={(e) => { if (!isControlled) setInternal(e.target.checked); onChange && onChange(e); }} {...rest} />
        <span className="cds-toggle__track" aria-hidden="true"><span className="cds-toggle__knob"></span></span>
        {!hideStateLabel ? <span className="cds-toggle__state">{on ? labelOn : labelOff}</span> : null}
      </label>
    </span>
  );
}
