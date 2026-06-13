import * as React from 'react';

/**
 * @startingPoint section="Forms" subtitle="Carbon text input field" viewport="700x360"
 */
export interface TextInputProps {
  /** Label shown above the field. */
  label?: string;
  id?: string;
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  type?: string;
  /** sm=32 md=40 lg=48. @default 'md' */
  size?: 'sm' | 'md' | 'lg';
  helperText?: string;
  invalid?: boolean;
  invalidText?: string;
  disabled?: boolean;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  className?: string;
}

/**
 * Single-line text field. Carbon fields have a filled background and a single
 * bottom rule — no full box — with the label above and helper/error below.
 */
export function TextInput(props: TextInputProps): JSX.Element;
