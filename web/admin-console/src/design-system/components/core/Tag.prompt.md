A small rounded pill for labels, categories, statuses and removable filters.

```jsx
<Tag type="blue">Running</Tag>
<Tag type="green" renderIcon={<img src="assets/icons/checkmark-filled.svg" width="16" />}>Healthy</Tag>
<Tag type="gray" filter onClose={() => {}}>us-south</Tag>
<Tag type="outline">Draft</Tag>
```

- `type`: any Carbon hue tint (`blue`, `green`, `red`, `purple`, `teal`, `magenta`, `cyan`, `gray`, `cool-gray`) or `outline`.
- `filter` adds a dismiss ✕; `size="sm"` for dense tables.
