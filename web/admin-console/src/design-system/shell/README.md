# Cloud Console — UI kit

A high-fidelity recreation of an **IBM Cloud–style console** built on the Carbon
UI Shell pattern. It demonstrates how the design system's primitives compose
into a real product surface.

## Run it
Open `index.html`. It loads the compiled `_ds_bundle.js` (Button, Tag, Tile,
Search, ProgressBar, InlineNotification…) plus two local view files.

## Surfaces
- **Header** (`shell.jsx`) — the 48px black global bar: menu, product name, top
  nav with an active blue underline, and right-aligned tool icons + avatar.
- **SideNav** (`shell.jsx`) — left navigation with an active item marked by a
  3px blue inset rule. Toggled by the header menu button.
- **DashboardView** (`views.jsx`) — page header with breadcrumb + primary action,
  an info notification, a 4-up metric strip, a compute-allocation tile with
  progress bars, and a resource **data table** with status tags and an overflow
  menu.
- **CatalogView** (`views.jsx`) — a searchable grid of clickable service tiles.

## Interactions
- Click **Catalog** in the side nav (or the header) to switch views.
- Type in the catalog search to filter service tiles live.
- The header menu button collapses / shows the side nav.

## Fidelity notes
This is a cosmetic recreation for prototyping, not production code. The data is
mocked and actions are inert. Layout, color, type, spacing and the shell anatomy
follow Carbon; behaviors are simplified.
