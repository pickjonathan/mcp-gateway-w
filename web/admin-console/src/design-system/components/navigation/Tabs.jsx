import React from 'react';

let injected = false;
function useTabsStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-tabs { font-family: var(--font-sans); width: 100%; }
  .cds-tabs__list { display: flex; list-style: none; margin: 0; padding: 0; box-shadow: inset 0 -1px 0 var(--border-subtle-01); }
  .cds-tab {
    position: relative; box-sizing: border-box;
    padding: var(--spacing-04) var(--spacing-05);
    min-width: 80px; max-width: 240px;
    background: transparent; border: none; cursor: pointer; text-align: left;
    font-family: var(--font-sans); font-size: 0.875rem; line-height: 1.28572; letter-spacing: 0.16px;
    color: var(--text-secondary);
    box-shadow: inset 0 -1px 0 var(--border-subtle-01);
    transition: color 70ms, box-shadow 70ms, background-color 70ms;
  }
  .cds-tab:hover { color: var(--text-primary); box-shadow: inset 0 -1px 0 var(--border-strong-01); }
  .cds-tab--selected { color: var(--text-primary); font-weight: 600; box-shadow: inset 0 -2px 0 var(--border-interactive); }
  .cds-tab:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
  .cds-tab:disabled { color: var(--text-disabled); cursor: not-allowed; }
  .cds-tabs__panel { padding: var(--spacing-05) 0; color: var(--text-primary); font-size: 0.875rem; line-height: 1.42857; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'tabs');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Tabs({ tabs = [], selectedIndex, defaultIndex = 0, onChange, className = '', children, ...rest }) {
  useTabsStyles();
  const [internal, setInternal] = React.useState(defaultIndex);
  const isControlled = selectedIndex !== undefined;
  const sel = isControlled ? selectedIndex : internal;
  const select = (i) => { if (!isControlled) setInternal(i); onChange && onChange(i); };
  const panels = React.Children.toArray(children);
  return (
    <div className={['cds-tabs', className].filter(Boolean).join(' ')} {...rest}>
      <ul className="cds-tabs__list" role="tablist">
        {tabs.map((t, i) => {
          const label = typeof t === 'string' ? t : t.label;
          const disabled = typeof t === 'object' && t.disabled;
          return (
            <li key={i} role="presentation">
              <button role="tab" aria-selected={sel === i} disabled={disabled}
                className={['cds-tab', sel === i ? 'cds-tab--selected' : ''].filter(Boolean).join(' ')}
                onClick={() => select(i)}>
                {label}
              </button>
            </li>
          );
        })}
      </ul>
      {panels.length ? <div className="cds-tabs__panel" role="tabpanel">{panels[sel]}</div> : null}
    </div>
  );
}
