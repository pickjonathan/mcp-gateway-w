import React from 'react';

let injected = false;
function useNotifStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-notif {
    display: flex; align-items: flex-start; gap: var(--spacing-05);
    box-sizing: border-box; min-height: 48px; width: 100%;
    padding: var(--spacing-04) var(--spacing-05);
    background: var(--layer-02); color: var(--text-primary);
    border-left: 3px solid var(--border-interactive); border-radius: 0;
    box-shadow: var(--shadow-sm);
    font-family: var(--font-sans);
  }
  .cds-notif--error { border-left-color: var(--support-error); background: var(--red-10); }
  .cds-notif--success { border-left-color: var(--support-success); background: var(--green-10); }
  .cds-notif--warning { border-left-color: var(--support-warning); background: var(--yellow-10); }
  .cds-notif--info { border-left-color: var(--support-info); background: var(--blue-10); }
  .cds-notif__icon { flex: none; width: 20px; height: 20px; margin-top: 2px; }
  .cds-notif--error .cds-notif__icon { color: var(--support-error); }
  .cds-notif--success .cds-notif__icon { color: var(--support-success); }
  .cds-notif--warning .cds-notif__icon { color: var(--support-warning); }
  .cds-notif--info .cds-notif__icon { color: var(--support-info); }
  .cds-notif__body { flex: 1; display: flex; flex-wrap: wrap; gap: 4px var(--spacing-03); }
  .cds-notif__title { font-size: 0.875rem; font-weight: 600; line-height: 1.42857; letter-spacing: 0.16px; }
  .cds-notif__subtitle { font-size: 0.875rem; line-height: 1.42857; letter-spacing: 0.16px; color: var(--text-primary); }
  .cds-notif__close { flex: none; display: grid; place-items: center; width: 24px; height: 24px; border: none; background: transparent; cursor: pointer; color: var(--icon-primary); }
  .cds-notif__close:hover { background: var(--background-hover); }
  .cds-notif__close svg { width: 16px; height: 16px; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'notif');
  el.textContent = css;
  document.head.appendChild(el);
}

const ICONS = {
  error: <path fill="currentColor" d="M10 1a9 9 0 109 9 9 9 0 00-9-9zm3.5 11.6L12.6 13.5 10 10.9l-2.6 2.6-.9-.9L9.1 10 6.5 7.4l.9-.9L10 9.1l2.6-2.6.9.9L10.9 10z"/>,
  success: <path fill="currentColor" d="M10 1a9 9 0 109 9 9 9 0 00-9-9zM8.7 13.5L5.2 10l.9-.9 2.6 2.6 5.2-5.2.9.9z"/>,
  warning: <path fill="currentColor" d="M10 1L1 18h18zm-.8 6h1.6v5H9.2zm.8 8.5a1 1 0 111-1 1 1 0 01-1 1z"/>,
  info: <path fill="currentColor" d="M10 1a9 9 0 109 9 9 9 0 00-9-9zm.8 13.5H9.2V9h1.6zM10 7.2a1 1 0 111-1 1 1 0 01-1 1z"/>,
};

export function InlineNotification({ kind = 'info', title, subtitle, onClose, hideClose = false, className = '', ...rest }) {
  useNotifStyles();
  return (
    <div className={['cds-notif', `cds-notif--${kind}`, className].filter(Boolean).join(' ')} role="status" {...rest}>
      <svg className="cds-notif__icon" viewBox="0 0 20 20" aria-hidden="true">{ICONS[kind]}</svg>
      <div className="cds-notif__body">
        {title ? <span className="cds-notif__title">{title}</span> : null}
        {subtitle ? <span className="cds-notif__subtitle">{subtitle}</span> : null}
      </div>
      {!hideClose ? (
        <button className="cds-notif__close" aria-label="Close" onClick={onClose}>
          <svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M12 4.7l-.7-.7L8 7.3 4.7 4l-.7.7L7.3 8 4 11.3l.7.7L8 8.7l3.3 3.3.7-.7L8.7 8z"/></svg>
        </button>
      ) : null}
    </div>
  );
}
