Carbon's base surface for grouping content — the building block of dashboards and product pages.

```jsx
<Tile>
  <h3 className="cds-type-heading-03">Cloud Foundry</h3>
  <p className="cds-type-body-01">Deploy apps without managing infrastructure.</p>
</Tile>

<Tile variant="clickable" href="#">Open service →</Tile>
<Tile variant="selectable" selected>Standard plan</Tile>
```

- `variant`: `base` (static), `clickable` (entire tile is a link/hover), `selectable` (single/multi select with a check).
- Tiles are sharp-cornered and sit on `--layer-01`. Compose freely with type utility classes.
