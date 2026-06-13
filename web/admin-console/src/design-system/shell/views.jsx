/* Cloud console main views: Dashboard (metrics + resource table),
   Catalog (service tiles). Consumes DS primitives from the bundle. */

const { Button, Tag, Tile, Search, Tabs, InlineNotification, ProgressBar } = window.CarbonDesignSystem_666911;

function MetricTile({ label, value, delta, deltaKind }) {
  return (
    <div className="ux-metric">
      <div className="ux-metric__label">{label}</div>
      <div className="ux-metric__value">{value}</div>
      {delta ? <div className={`ux-metric__delta ux-metric__delta--${deltaKind}`}>{delta}</div> : null}
    </div>
  );
}

const RESOURCES = [
  { name: 'inference-api-prod', type: 'Code Engine', region: 'us-south', status: 'Running', s: 'green' },
  { name: 'vpc-mzr-dallas', type: 'VPC Infrastructure', region: 'us-south', status: 'Running', s: 'green' },
  { name: 'pg-orders-db', type: 'Databases for PostgreSQL', region: 'eu-de', status: 'Provisioning', s: 'blue' },
  { name: 'cos-model-weights', type: 'Object Storage', region: 'global', status: 'Running', s: 'green' },
  { name: 'kube-staging-01', type: 'Kubernetes Service', region: 'jp-tok', status: 'Degraded', s: 'red' },
  { name: 'watsonx-assistant', type: 'watsonx Assistant', region: 'us-south', status: 'Running', s: 'green' },
];

function ResourceTable({ query }) {
  const rows = RESOURCES.filter((r) => !query || r.name.toLowerCase().includes(query.toLowerCase()) || r.type.toLowerCase().includes(query.toLowerCase()));
  return (
    <div className="ux-table">
      <div className="ux-table__toolbar">
        <div className="ux-table__search"><Search placeholder="Search resources" defaultValue="" onChange={() => {}} /></div>
        <Button kind="ghost" size="md" renderIcon={<img src="../../assets/icons/filter.svg" width="16" alt="" />}>Filter</Button>
        <Button kind="primary" size="md" renderIcon={<img src="../../assets/icons/add.svg" width="16" alt="" />}>Create</Button>
      </div>
      <table>
        <thead>
          <tr>
            <th>Name</th><th>Service</th><th>Location</th><th>Status</th><th aria-label="Actions"></th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name}>
              <td className="ux-table__name">{r.name}</td>
              <td>{r.type}</td>
              <td>{r.region}</td>
              <td><Tag type={r.s} size="sm">{r.status}</Tag></td>
              <td className="ux-table__actions"><button aria-label="Actions"><img src="../../assets/icons/overflow-menu-vertical.svg" width="16" alt="" /></button></td>
            </tr>
          ))}
          {rows.length === 0 ? <tr><td colSpan="5" className="ux-table__empty">No resources match “{query}”.</td></tr> : null}
        </tbody>
      </table>
    </div>
  );
}

function DashboardView() {
  const [query, setQuery] = React.useState('');
  return (
    <div className="ux-view">
      <div className="ux-page-head">
        <div>
          <div className="ux-breadcrumb">Home / Resource list</div>
          <h1 className="cds-type-fluid-heading-05">Dashboard</h1>
        </div>
        <Button kind="primary" renderIcon={<img src="../../assets/icons/add.svg" width="16" alt="" />}>Create resource</Button>
      </div>

      <div className="ux-notif-wrap">
        <InlineNotification kind="info" title="watsonx.ai is now available in eu-de" subtitle="Provision a new project to get started." />
      </div>

      <div className="ux-metrics">
        <MetricTile label="Running services" value="24" delta="▲ 3 this week" deltaKind="up" />
        <MetricTile label="Monthly spend" value="$1,284" delta="▲ 12%" deltaKind="up" />
        <MetricTile label="Open incidents" value="1" delta="▼ 2" deltaKind="down" />
        <MetricTile label="Compute used" value="64%" />
      </div>

      <div className="ux-usage">
        <Tile>
          <div className="ux-usage__head"><strong className="cds-type-heading-compact-02">Compute allocation</strong><Tag type="blue" size="sm">us-south</Tag></div>
          <div className="ux-usage__bars">
            <ProgressBar label="vCPU" value={64} helperText="128 of 200 vCPU" />
            <ProgressBar label="Memory" value={48} helperText="384 of 800 GB" />
            <ProgressBar label="Storage" value={82} status="error" helperText="4.1 of 5 TB — near limit" />
          </div>
        </Tile>
      </div>

      <h2 className="cds-type-heading-03 ux-section-title">Your resources</h2>
      <ResourceTable query={query} />
    </div>
  );
}

const CATALOG = [
  { name: 'Code Engine', cat: 'Compute', desc: 'Run containers, apps and jobs on a fully managed platform.', icon: 'ibm-cloud', tag: 'Compute' },
  { name: 'Kubernetes Service', cat: 'Containers', desc: 'Deploy secure, highly available clusters in minutes.', icon: 'dashboard', tag: 'Containers' },
  { name: 'Databases for PostgreSQL', cat: 'Databases', desc: 'A managed, enterprise-ready PostgreSQL with HA.', icon: 'folder', tag: 'Databases' },
  { name: 'watsonx.ai', cat: 'AI / ML', desc: 'Train, tune and deploy foundation models.', icon: 'analytics', tag: 'AI / ML' },
  { name: 'Object Storage', cat: 'Storage', desc: 'Durable, scalable storage for unstructured data.', icon: 'document', tag: 'Storage' },
  { name: 'Secrets Manager', cat: 'Security', desc: 'Create, lease and centrally manage secrets.', icon: 'locked', tag: 'Security' },
];

function CatalogView() {
  const [q, setQ] = React.useState('');
  const list = CATALOG.filter((c) => !q || c.name.toLowerCase().includes(q.toLowerCase()));
  return (
    <div className="ux-view">
      <div className="ux-page-head">
        <div>
          <div className="ux-breadcrumb">Home / Catalog</div>
          <h1 className="cds-type-fluid-heading-05">Catalog</h1>
        </div>
      </div>
      <div className="ux-catalog__search"><Search placeholder="Search the catalog" onChange={(e) => setQ(e.target.value)} /></div>
      <div className="ux-catalog">
        {list.map((c) => (
          <Tile key={c.name} variant="clickable" href="#">
            <div className="ux-cat__top">
              <span className="ux-cat__icon"><img src={`../../assets/icons/${c.icon}.svg`} width="24" height="24" alt="" /></span>
              <Tag type="cool-gray" size="sm">{c.tag}</Tag>
            </div>
            <strong className="cds-type-heading-compact-02 ux-cat__name">{c.name}</strong>
            <p className="cds-type-body-01 ux-cat__desc">{c.desc}</p>
          </Tile>
        ))}
      </div>
    </div>
  );
}

Object.assign(window, { DashboardView, CatalogView, ResourceTable });
