import * as React from 'react';

/**
 * @startingPoint section="Core" subtitle="Carbon tile / card surface" viewport="700x340"
 */
export interface TileProps {
  children?: React.ReactNode;
  /** base = static container, clickable = whole-tile link, selectable = toggle. @default 'base' */
  variant?: 'base' | 'clickable' | 'selectable';
  /** Selected state for selectable tiles. */
  selected?: boolean;
  href?: string;
  onClick?: (e: React.MouseEvent) => void;
  className?: string;
}

/**
 * Carbon's fundamental surface. A sharp-cornered container on the layer-01
 * background that groups related content. Clickable and selectable variants
 * add hover and selection affordances.
 */
export function Tile(props: TileProps): JSX.Element;
