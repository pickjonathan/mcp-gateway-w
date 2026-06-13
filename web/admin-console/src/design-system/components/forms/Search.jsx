import React from 'react';

let injected = false;
function useSearchStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-search { position: relative; display: flex; align-items: center; width: 100%; font-family: var(--font-sans); }
  .cds-search__icon { position: absolute; left: var(--spacing-05); width: 16px; height: 16px; color: var(--icon-secondary); pointer-events: none; }
  .cds-search__input {
    box-sizing: border-box; width: 100%; height: 40px;
    padding: 0 40px 0 44px; margin: 0;
    background: var(--field-01); color: var(--text-primary);
    border: none; border-bottom: 1px solid var(--border-strong-01); border-radius: 0;
    font-family: var(--font-sans); font-size: 0.875rem; letter-spacing: 0.16px;
    outline: 2px solid transparent; outline-offset: -2px;
    transition: background-color 70ms var(--easing-standard-productive);
  }
  .cds-search__input::placeholder { color: var(--text-placeholder); }
  .cds-search__input:hover { background: var(--field-hover-01); }
  .cds-search__input:focus { outline: 2px solid var(--focus); }
  .cds-search--sm .cds-search__input { height: 32px; }
  .cds-search--lg .cds-search__input { height: 48px; }
  .cds-search__clear { position: absolute; right: 0; display: grid; place-items: center; width: 40px; height: 100%; border: none; background: transparent; cursor: pointer; color: var(--icon-primary); opacity: 0; pointer-events: none; }
  .cds-search--has-value .cds-search__clear { opacity: 1; pointer-events: auto; }
  .cds-search__clear:hover { background: var(--field-hover-01); }
  .cds-search__clear svg { width: 16px; height: 16px; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'search');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Search({ placeholder = 'Search', value, defaultValue = '', size = 'md', onChange, onClear, className = '', ...rest }) {
  useSearchStyles();
  const [internal, setInternal] = React.useState(defaultValue);
  const isControlled = value !== undefined;
  const v = isControlled ? value : internal;
  const cls = ['cds-search', size === 'sm' ? 'cds-search--sm' : size === 'lg' ? 'cds-search--lg' : '', v ? 'cds-search--has-value' : '', className].filter(Boolean).join(' ');
  return (
    <div className={cls}>
      <svg className="cds-search__icon" viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M15 14.3L10.7 10a6 6 0 10-.7.7l4.3 4.3.7-.7zM2 6.5A4.5 4.5 0 116.5 11 4.5 4.5 0 012 6.5z"/></svg>
      <input className="cds-search__input" type="text" placeholder={placeholder} value={v}
        onChange={(e) => { if (!isControlled) setInternal(e.target.value); onChange && onChange(e); }} {...rest} />
      <button className="cds-search__clear" aria-label="Clear" onClick={() => { if (!isControlled) setInternal(''); onClear && onClear(); }}>
        <svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M12 4.7l-.7-.7L8 7.3 4.7 4l-.7.7L7.3 8 4 11.3l.7.7L8 8.7l3.3 3.3.7-.7L8.7 8z"/></svg>
      </button>
    </div>
  );
}
