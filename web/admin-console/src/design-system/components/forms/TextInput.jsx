import React from 'react';

let injected = false;
function useFieldStyles() {
  if (injected || typeof document === 'undefined') return;
  injected = true;
  const css = `
  .cds-field { display: flex; flex-direction: column; font-family: var(--font-sans); }
  .cds-field__label { font-size: 0.75rem; line-height: 1.33333; letter-spacing: 0.32px; color: var(--text-secondary); margin-bottom: var(--spacing-03); }
  .cds-field__wrap { position: relative; display: flex; align-items: center; }
  .cds-input {
    box-sizing: border-box; width: 100%; height: 40px;
    padding: 0 var(--spacing-05); margin: 0;
    background: var(--field-01); color: var(--text-primary);
    border: none; border-bottom: 1px solid var(--border-strong-01); border-radius: 0;
    font-family: var(--font-sans); font-size: 0.875rem; letter-spacing: 0.16px;
    transition: background-color 70ms var(--easing-standard-productive), outline 70ms var(--easing-standard-productive);
    outline: 2px solid transparent; outline-offset: -2px;
  }
  .cds-input::placeholder { color: var(--text-placeholder); }
  .cds-input:hover { background: var(--field-hover-01); }
  .cds-input:focus { outline: 2px solid var(--focus); }
  .cds-input--sm { height: 32px; }
  .cds-input--lg { height: 48px; }
  .cds-input--invalid { outline: 2px solid var(--support-error); }
  .cds-field__helper { font-size: 0.75rem; line-height: 1.33333; letter-spacing: 0.32px; color: var(--text-helper); margin-top: var(--spacing-02); }
  .cds-field__error { font-size: 0.75rem; line-height: 1.33333; letter-spacing: 0.32px; color: var(--text-error); margin-top: var(--spacing-02); }
  .cds-input:disabled { color: var(--text-disabled); border-bottom-color: transparent; cursor: not-allowed; }
  `;
  const el = document.createElement('style');
  el.setAttribute('data-cds', 'field');
  el.textContent = css;
  document.head.appendChild(el);
}

export function TextInput({
  label, id, value, defaultValue, placeholder, type = 'text',
  size = 'md', helperText, invalid = false, invalidText, disabled = false,
  onChange, className = '', ...rest
}) {
  useFieldStyles();
  const inputId = id || `cds-input-${Math.random().toString(36).slice(2, 8)}`;
  const inputCls = [
    'cds-input',
    size === 'sm' ? 'cds-input--sm' : size === 'lg' ? 'cds-input--lg' : '',
    invalid ? 'cds-input--invalid' : '',
  ].filter(Boolean).join(' ');
  return (
    <div className={['cds-field', className].filter(Boolean).join(' ')}>
      {label ? <label className="cds-field__label" htmlFor={inputId}>{label}</label> : null}
      <div className="cds-field__wrap">
        <input
          id={inputId} className={inputCls} type={type} value={value}
          defaultValue={defaultValue} placeholder={placeholder} disabled={disabled}
          onChange={onChange} {...rest}
        />
      </div>
      {invalid && invalidText ? <div className="cds-field__error">{invalidText}</div>
        : helperText ? <div className="cds-field__helper">{helperText}</div> : null}
    </div>
  );
}
