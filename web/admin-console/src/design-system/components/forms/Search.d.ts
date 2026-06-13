import * as React from 'react';

export interface SearchProps {
  placeholder?: string;
  value?: string;
  defaultValue?: string;
  size?: 'sm' | 'md' | 'lg';
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onClear?: () => void;
  className?: string;
}

/**
 * Carbon search field — leading magnifier, filled background, single bottom
 * rule, and a clear button that appears once there's a value.
 */
export function Search(props: SearchProps): JSX.Element;
