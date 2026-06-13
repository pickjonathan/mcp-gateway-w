import React from 'react';

let injected = false;
function useTileStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-tile {
    position: relative; box-sizing: border-box; display: block;
    min-height: 64px; padding: var(--spacing-05);
    background: var(--layer-01); color: var(--text-primary);
    border-radius: 0; border: 1px solid transparent;
    font-family: var(--font-sans);
  }
  .cds-tile--clickable { cursor: pointer; text-decoration: none; transition: background-color 70ms var(--easing-standard-productive); }
  .cds-tile--clickable:hover { background: var(--layer-hover-01); }
  .cds-tile--clickable:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
  .cds-tile--selectable { border: 1px solid var(--border-tile-01); }
  .cds-tile--selected { border-color: var(--gray-100); box-shadow: inset 0 0 0 1px var(--gray-100); }
  .cds-tile__check { position: absolute; top: var(--spacing-05); right: var(--spacing-05); width: 16px; height: 16px; color: var(--gray-100); opacity: 0; }
  .cds-tile--selected .cds-tile__check { opacity: 1; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'tile');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Tile({ children, variant = 'base', selected = false, href, onClick, className = '', ...rest }) {
  useTileStyles();
  const clickable = variant === 'clickable' || !!href;
  const cls = [
    'cds-tile',
    clickable ? 'cds-tile--clickable' : '',
    variant === 'selectable' ? 'cds-tile--selectable' : '',
    variant === 'selectable' && selected ? 'cds-tile--selected' : '',
    className,
  ].filter(Boolean).join(' ');

  const check = variant === 'selectable' ? (
    <span className="cds-tile__check" aria-hidden="true">✓</span>
  ) : null;

  if (href) {
    return <a className={cls} href={href} onClick={onClick} {...rest}>{children}{check}</a>;
  }
  return <div className={cls} onClick={onClick} {...rest}>{children}{check}</div>;
}
