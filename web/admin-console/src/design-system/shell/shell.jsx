/* Cloud console UI-shell pieces: top Header + left SideNav.
   Carbon UI Shell = a 48px black (gray-100) global bar over a
   light productive workspace. */

const Icon = ({ n, s = 16, color }) => (
  <img src={`../../assets/icons/${n}.svg`} width={s} height={s} alt="" style={color ? { filter: 'none' } : undefined} />
);

function Header({ onMenu, productName = 'IBM Cloud' }) {
  return (
    <header className="ux-header">
      <button className="ux-header__action" aria-label="Open menu" onClick={onMenu}>
        <img src="../../assets/icons/menu.svg" width="20" height="20" alt="" className="ux-ico-inv" />
      </button>
      <a className="ux-header__name" href="#"><b>IBM</b>&nbsp;Cloud</a>
      <nav className="ux-header__nav">
        <a className="ux-header__navitem ux-header__navitem--active" href="#">Catalog</a>
        <a className="ux-header__navitem" href="#">Docs</a>
        <a className="ux-header__navitem" href="#">Manage</a>
      </nav>
      <div className="ux-header__spacer"></div>
      <div className="ux-header__tools">
        <button className="ux-header__action" aria-label="Search"><img src="../../assets/icons/search.svg" width="20" height="20" className="ux-ico-inv" alt="" /></button>
        <button className="ux-header__action" aria-label="Notifications"><img src="../../assets/icons/notification.svg" width="20" height="20" className="ux-ico-inv" alt="" /></button>
        <button className="ux-header__action" aria-label="Settings"><img src="../../assets/icons/settings.svg" width="20" height="20" className="ux-ico-inv" alt="" /></button>
        <button className="ux-header__action" aria-label="Apps"><img src="../../assets/icons/switcher.svg" width="20" height="20" className="ux-ico-inv" alt="" /></button>
        <button className="ux-header__action ux-header__avatar" aria-label="Account"><img src="../../assets/icons/user-avatar.svg" width="20" height="20" className="ux-ico-inv" alt="" /></button>
      </div>
    </header>
  );
}

const NAV = [
  { id: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
  { id: 'catalog', label: 'Catalog', icon: 'ibm-cloud' },
  { id: 'resources', label: 'Resource list', icon: 'list-bulleted' },
  { id: 'data', label: 'Databases', icon: 'folder' },
  { id: 'observability', label: 'Observability', icon: 'analytics' },
  { id: 'security', label: 'Security', icon: 'locked' },
];

function SideNav({ open, current, onSelect }) {
  return (
    <aside className={`ux-sidenav ${open ? 'is-open' : ''}`}>
      <nav>
        <ul className="ux-sidenav__list">
          {NAV.map((item) => (
            <li key={item.id}>
              <button
                className={`ux-sidenav__link ${current === item.id ? 'is-active' : ''}`}
                onClick={() => onSelect(item.id)}>
                <img src={`../../assets/icons/${item.icon}.svg`} width="16" height="16" alt="" className="ux-sidenav__icon" />
                <span>{item.label}</span>
              </button>
            </li>
          ))}
        </ul>
        <div className="ux-sidenav__divider"></div>
        <ul className="ux-sidenav__list">
          <li><button className="ux-sidenav__link" onClick={() => onSelect('docs')}><img src="../../assets/icons/document.svg" width="16" height="16" alt="" className="ux-sidenav__icon" /><span>Documentation</span></button></li>
          <li><button className="ux-sidenav__link" onClick={() => onSelect('support')}><img src="../../assets/icons/help.svg" width="16" height="16" alt="" className="ux-sidenav__icon" /><span>Support</span></button></li>
        </ul>
      </nav>
    </aside>
  );
}

Object.assign(window, { Header, SideNav, Icon, NAV });
