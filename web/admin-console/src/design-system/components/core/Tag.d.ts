import * as React from 'react';

export interface TagProps {
  children?: React.ReactNode;
  /** Color variant. @default 'gray' */
  type?: 'gray' | 'cool-gray' | 'blue' | 'green' | 'red' | 'purple' | 'teal' | 'magenta' | 'cyan' | 'outline';
  /** @default 'md' */
  size?: 'sm' | 'md';
  /** Show a dismiss button (filter / chip tag). */
  filter?: boolean;
  /** Optional leading icon node. */
  renderIcon?: React.ReactNode;
  onClose?: (e: React.MouseEvent) => void;
  className?: string;
}

/**
 * A small pill that labels, categorizes or filters. Carbon tags are fully
 * rounded with a low-saturation tint background and a darker text of the
 * same hue. Use `filter` for removable selections.
 */
export function Tag(props: TagProps): JSX.Element;
