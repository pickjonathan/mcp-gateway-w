import React from 'react';

/**
 * Carbon Button.
 * Sharp-cornered, asymmetric-padding button with the five Carbon kinds.
 * Styling is driven entirely by Carbon CSS custom properties (link the
 * design system's styles.css on the page).
 */

let injected = false;
function useButtonStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-btn {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--spacing-05);
    box-sizing: border-box;
    max-width: 320px;
    min-height: 48px;
    margin: 0;
    padding: 0 63px 0 15px;
    border: 1px solid transparent;
    border-radius: 0;
    font-family: var(--font-sans);
    font-size: 0.875rem;
    font-weight: 400;
    line-height: 1.28572;
    letter-spacing: 0.16px;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
    transition: background-color 70ms var(--easing-standard-productive),
                box-shadow 70ms var(--easing-standard-productive),
                border-color 70ms var(--easing-standard-productive),
                color 70ms var(--easing-standard-productive);
  }
  .cds-btn:focus-visible { outline: none; border-color: var(--focus); box-shadow: inset 0 0 0 1px var(--focus), inset 0 0 0 2px var(--focus-inset); }
  .cds-btn__icon { flex: 0 0 auto; width: 16px; height: 16px; }
  .cds-btn--icon-only { padding: 0; width: 48px; justify-content: center; }
  .cds-btn--icon-only .cds-btn__icon { width: 20px; height: 20px; }

  /* sizes */
  .cds-btn--sm { min-height: 32px; padding-right: 47px; }
  .cds-btn--md { min-height: 40px; padding-right: 47px; }
  .cds-btn--lg { min-height: 48px; }
  .cds-btn--xl { min-height: 64px; align-items: flex-start; padding-top: 14px; }
  .cds-btn--sm.cds-btn--icon-only { width: 32px; }
  .cds-btn--md.cds-btn--icon-only { width: 40px; }

  /* primary */
  .cds-btn--primary { background: var(--button-primary); color: var(--text-on-color); }
  .cds-btn--primary:hover { background: var(--button-primary-hover); }
  .cds-btn--primary:active { background: var(--button-primary-active); }

  /* secondary */
  .cds-btn--secondary { background: var(--button-secondary); color: var(--text-on-color); }
  .cds-btn--secondary:hover { background: var(--button-secondary-hover); }
  .cds-btn--secondary:active { background: var(--button-secondary-active); }

  /* tertiary */
  .cds-btn--tertiary { background: transparent; color: var(--button-tertiary); border-color: var(--button-tertiary); }
  .cds-btn--tertiary:hover { background: var(--button-primary-hover); color: var(--text-on-color); border-color: var(--button-primary-hover); }
  .cds-btn--tertiary:active { background: var(--button-primary-active); border-color: var(--button-primary-active); color: var(--text-on-color); }

  /* ghost */
  .cds-btn--ghost { background: transparent; color: var(--link-primary); padding-right: 15px; }
  .cds-btn--ghost:hover { background: var(--background-hover); }
  .cds-btn--ghost:active { background: var(--background-active); }

  /* danger */
  .cds-btn--danger { background: var(--button-danger); color: var(--text-on-color); }
  .cds-btn--danger:hover { background: var(--button-danger-hover); }
  .cds-btn--danger:active { background: var(--button-danger-active); }
  .cds-btn--danger-tertiary { background: transparent; color: var(--button-danger); border-color: var(--button-danger); }
  .cds-btn--danger-tertiary:hover { background: var(--button-danger-hover); color: var(--text-on-color); border-color: var(--button-danger-hover); }

  .cds-btn:disabled, .cds-btn--disabled {
    background: var(--button-disabled); color: var(--text-on-color-disabled);
    border-color: transparent; cursor: not-allowed; pointer-events: none;
  }
  .cds-btn--ghost:disabled, .cds-btn--tertiary:disabled {
    background: transparent; color: var(--text-disabled); border-color: var(--border-disabled);
  }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'button');
  el.textContent = css;
  document.head.appendChild(el);
}

export function Button({
  children,
  kind = 'primary',
  size = 'lg',
  disabled = false,
  iconOnly = false,
  renderIcon = null,
  href,
  onClick,
  type = 'button',
  className = '',
  ...rest
}) {
  useButtonStyles();
  const cls = [
    'cds-btn',
    `cds-btn--${kind}`,
    `cds-btn--${size}`,
    iconOnly ? 'cds-btn--icon-only' : '',
    disabled ? 'cds-btn--disabled' : '',
    className,
  ].filter(Boolean).join(' ');

  const icon = renderIcon ? <span className="cds-btn__icon" aria-hidden="true">{renderIcon}</span> : null;
  const content = iconOnly ? icon : (<>{children}{icon}</>);

  if (href && !disabled) {
    return <a className={cls} href={href} onClick={onClick} {...rest}>{content}</a>;
  }
  return (
    <button className={cls} type={type} disabled={disabled} onClick={onClick} {...rest}>
      {content}
    </button>
  );
}
