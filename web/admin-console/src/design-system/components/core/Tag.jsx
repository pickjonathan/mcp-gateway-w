import React from 'react';

let injected = false;
function useTagStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-tag {
    display: inline-flex; align-items: center; gap: var(--spacing-02);
    box-sizing: border-box; height: 24px; max-width: 100%;
    padding: 0 var(--spacing-03); margin: 0;
    border: none; border-radius: 999px;
    font-family: var(--font-sans); font-size: 0.75rem; font-weight: 400;
    line-height: 1.33333; letter-spacing: 0.32px; white-space: nowrap;
  }
  .cds-tag--sm { height: 18px; }
  .cds-tag__icon { width: 16px; height: 16px; flex: none; }
  .cds-tag--gray { background: var(--gray-20); color: var(--gray-100); }
  .cds-tag--blue { background: var(--blue-20); color: var(--blue-70); }
  .cds-tag--green { background: var(--green-20); color: var(--green-70); }
  .cds-tag--red { background: var(--red-20); color: var(--red-70); }
  .cds-tag--purple { background: var(--purple-20); color: var(--purple-70); }
  .cds-tag--teal { background: var(--teal-20); color: var(--teal-70); }
  .cds-tag--magenta { background: var(--magenta-20); color: var(--magenta-70); }
  .cds-tag--cyan { background: var(--cyan-20); color: var(--cyan-70); }
  .cds-tag--cool-gray { background: var(--cool-gray-20); color: var(--cool-gray-70); }
  .cds-tag--outline { background: transparent; box-shadow: inset 0 0 0 1px var(--border-strong-01); color: var(--text-primary); }
  .cds-tag--filter { padding-right: var(--spacing-02); }
  .cds-tag__close { display: inline-grid; place-items: center; width: 18px; height: 18px; margin-left: var(--spacing-02); border: none; background: transparent; border-radius: 999px; cursor: pointer; color: inherit; }
  .cds-tag__close:hover { background: rgba(0,0,0,0.12); }
  .cds-tag__close svg, .cds-tag__close img { width: 12px; height: 12px; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'tag');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Tag({ children, type = 'gray', size = 'md', filter = false, renderIcon = null, onClose, className = '', ...rest }) {
  useTagStyles();
  const cls = ['cds-tag', `cds-tag--${type}`, size === 'sm' ? 'cds-tag--sm' : '', filter ? 'cds-tag--filter' : '', className].filter(Boolean).join(' ');
  return (
    <span className={cls} {...rest}>
      {renderIcon ? <span className="cds-tag__icon" aria-hidden="true">{renderIcon}</span> : null}
      {children}
      {filter ? (
        <button className="cds-tag__close" aria-label="Dismiss" onClick={onClose}>✕</button>
      ) : null}
    </span>
  );
}
