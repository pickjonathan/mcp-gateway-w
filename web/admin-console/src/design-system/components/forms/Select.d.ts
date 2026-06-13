import * as React from 'react';

export interface SelectProps {
  label?: string;
  children?: React.ReactNode;
  value?: string;
  defaultValue?: string;
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  onChange?: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  id?: string;
  className?: string;
}

/**
 * Native select styled as a Carbon dropdown field — filled background, single
 * bottom rule, trailing chevron. Pass <option> children.
 */
export function Select(props: SelectProps): JSX.Element;
