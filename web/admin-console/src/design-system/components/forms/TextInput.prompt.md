Single-line text entry with label, helper text and validation.

```jsx
<TextInput label="Email address" placeholder="you@example.com" helperText="We'll never share it." />
<TextInput label="API key" invalid invalidText="This key is no longer valid." />
```

- Carbon fields are filled (`--field-01`) with one bottom border; the box is intentionally open.
- `size` sm/md/lg; pass `invalid` + `invalidText` for the error treatment.
