Carbon's main action element — use for any clickable action; pick the `kind` to express hierarchy.

```jsx
<Button kind="primary" renderIcon={<img src="assets/icons/arrow-right.svg" width="16" />}>
  Get started
</Button>
<Button kind="secondary">Cancel</Button>
<Button kind="ghost" size="sm">Skip</Button>
<Button kind="danger" renderIcon={<img src="assets/icons/trash-can.svg" width="16" />}>Delete</Button>
<Button iconOnly kind="ghost" renderIcon={<img src="assets/icons/settings.svg" width="20" />} aria-label="Settings" />
```

- `kind`: `primary` (one per view), `secondary`, `tertiary` (outline), `ghost` (text-only), `danger`, `danger-tertiary`.
- `size`: `sm` 32 · `md` 40 · `lg` 48 (default) · `xl` 64.
- Carbon convention: trailing icon, sharp corners, asymmetric right padding. Use `iconOnly` for toolbar actions.
