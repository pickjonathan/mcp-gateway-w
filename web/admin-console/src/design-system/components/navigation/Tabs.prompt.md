Switch between related views within the same context.

```jsx
<Tabs tabs={['Overview', 'Usage', 'Settings']}>
  <div>Overview panel…</div>
  <div>Usage panel…</div>
  <div>Settings panel…</div>
</Tabs>
```

- Selected tab shows a 2px `--border-interactive` underline. Pass `{ label, disabled }` objects to disable a tab.
