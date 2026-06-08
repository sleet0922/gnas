// GNAS - Material UI SPA
const API = '';
let token = null;

// --- HTTP helpers ---
async function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  const res = await fetch(API + path, { ...opts, headers });
  if (res.status === 401) { navigate('login'); return null; }
  const data = await res.json();
  if (data.code !== 0 && data.code !== undefined) {
    if (opts.silent !== true) toast(data.message || '操作失败');
    return null;
  }
  return data.data;
}

async function apiGet(path) { return api(path, { silent: true }); }
async function apiPost(path, body, opts = {}) {
  return api(path, { method: 'POST', body: JSON.stringify(body), ...opts });
}

// --- Toast ---
function toast(msg, duration = 3000) {
  const el = document.createElement('div');
  el.className = 'toast'; el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), duration);
}

// --- Router ---
let currentPage = 'dashboard';
function navigate(page) {
  currentPage = page;
  window.location.hash = page;
  render();
}

// --- App ---
window.addEventListener('hashchange', () => {
  const hash = window.location.hash.slice(1) || 'dashboard';
  if (hash !== currentPage) { currentPage = hash; render(); }
});

window.addEventListener('DOMContentLoaded', () => {
  currentPage = window.location.hash.slice(1) || 'dashboard';
  render();
});

async function render() {
  const app = document.getElementById('app');
  // Check auth
  if (currentPage !== 'login') {
    const status = await apiGet('/api/status');
    if (!status) { currentPage = 'login'; }
  }

  switch (currentPage) {
    case 'login': app.innerHTML = renderLoginPage(); bindLogin(); break;
    case 'dashboard': app.innerHTML = renderLayout(renderDashboard()); break;
    case 'ddns': app.innerHTML = renderLayout(await renderDDNS()); bindDDNS(); break;
    case 'logs': app.innerHTML = renderLayout(await renderLogs()); bindLogs(); break;
    default: app.innerHTML = renderLayout(renderDashboard()); break;
  }
}

// --- Layout ---
function renderLayout(content) {
  const items = [
    { id: 'dashboard', icon: 'dashboard', label: '概览' },
    { id: 'ddns', icon: 'dns', label: 'DDNS' },
    { id: 'logs', icon: 'terminal', label: '日志' },
  ];
  const navItems = items.map(i => `
    <div class="nav-rail-item ${currentPage === i.id ? 'active' : ''}" onclick="navigate('${i.id}')">
      <div class="icon-wrap"><span class="material-icons-round">${i.icon}</span></div>
      <span class="label">${i.label}</span>
    </div>
  `).join('');
  const bottomItems = items.map(i => `
    <div class="bottom-nav-item ${currentPage === i.id ? 'active' : ''}" onclick="navigate('${i.id}')">
      <span class="material-icons-round">${i.icon}</span>
      <span>${i.label}</span>
    </div>
  `).join('');

  return `
    <nav class="nav-rail">
      <div class="logo">G</div>
      ${navItems}
      <div class="nav-rail-spacer"></div>
      <div class="nav-rail-item" onclick="doLogout()">
        <div class="icon-wrap"><span class="material-icons-round">logout</span></div>
        <span class="label">登出</span>
      </div>
    </nav>
    <main class="main-content">${content}</main>
    <div class="bottom-nav">${bottomItems}</div>
  `;
}

// --- Dashboard ---
function renderDashboard() {
  return `
    <div class="top-bar"><h1>概览</h1></div>
    <div class="card">
      <div class="card-title"><span class="material-icons-round">info</span>系统状态</div>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
        <div>
          <div style="font-size:12px;color:var(--md-on-surface-variant);">DDNS 服务</div>
          <div style="font-size:16px;font-weight:500;display:flex;align-items:center;gap:8px;">
            <span class="status-dot running"></span>运行中
          </div>
        </div>
        <div>
          <div style="font-size:12px;color:var(--md-on-surface-variant);">版本</div>
          <div style="font-size:16px;font-weight:500;">1.0.0</div>
        </div>
      </div>
    </div>
    <div class="card" style="cursor:pointer;" onclick="navigate('ddns')">
      <div class="card-title"><span class="material-icons-round">dns</span>DDNS 配置</div>
      <p style="font-size:14px;color:var(--md-on-surface-variant);">管理动态域名解析配置，支持多种 DNS 服务商</p>
    </div>
    <div class="card" style="cursor:pointer;" onclick="navigate('logs')">
      <div class="card-title"><span class="material-icons-round">terminal</span>运行日志</div>
      <p style="font-size:14px;color:var(--md-on-surface-variant);">查看 DDNS 运行日志和更新记录</p>
    </div>
  `;
}

// --- Login ---
function renderLoginPage() {
  return `
    <div class="login-page">
      <div class="login-card">
        <div class="logo">G</div>
        <h2>GNAS</h2>
        <p>登录以管理你的 NAS 服务</p>
        <div class="form-group">
          <label>用户名</label>
          <input class="md-input" id="login-user" type="text" placeholder="输入用户名" autocomplete="username">
        </div>
        <div class="form-group">
          <label>密码</label>
          <input class="md-input" id="login-pass" type="password" placeholder="输入密码" autocomplete="current-password">
        </div>
        <button class="md-btn md-btn-filled" id="login-btn">登录</button>
      </div>
    </div>
  `;
}

function bindLogin() {
  document.getElementById('login-btn')?.addEventListener('click', async () => {
    const username = document.getElementById('login-user').value;
    const password = document.getElementById('login-pass').value;
    const res = await apiPost('/api/login', { username, password });
    if (res) { token = res.token; navigate('dashboard'); }
  });
  document.getElementById('login-pass')?.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') document.getElementById('login-btn')?.click();
  });
}

async function doLogout() {
  await apiPost('/api/logout', {});
  token = null;
  navigate('login');
}

// --- DDNS ---
let ddnsConfig = null;
let activeDnsTab = 0;

const DNS_PROVIDERS = [
  { value: 'alidns', label: '阿里云' },
  { value: 'aliesa', label: '阿里云 ESA' },
  { value: 'tencentcloud', label: '腾讯云' },
  { value: 'dnspod', label: 'Dnspod' },
  { value: 'cloudflare', label: 'Cloudflare' },
  { value: 'huaweicloud', label: '华为云' },
  { value: 'callback', label: 'Callback' },
  { value: 'baiducloud', label: '百度云' },
  { value: 'porkbun', label: 'Porkbun' },
  { value: 'godaddy', label: 'GoDaddy' },
  { value: 'namecheap', label: 'Namecheap' },
  { value: 'namesilo', label: 'NameSilo' },
  { value: 'dynadot', label: 'Dynadot' },
  { value: 'vercel', label: 'Vercel' },
  { value: 'gcore', label: 'Gcore' },
  { value: 'edgeone', label: 'EdgeOne' },
  { value: 'rainyun', label: '雨云' },
  { value: 'cloudns', label: 'ClouDNS' },
  { value: 'dnsla', label: 'DNSLA' },
];

async function renderDDNS() {
  const data = await apiGet('/api/config');
  if (!data) return '<p>加载配置失败</p>';
  ddnsConfig = data;
  activeDnsTab = 0;

  const dnsTabs = (data.dnsConf || []).map((_, i) =>
    `<button class="dns-tab ${i === 0 ? 'active' : ''}" data-tab="${i}">配置 ${i + 1}</button>`
  ).join('');

  return `
    <div class="top-bar"><h1>DDNS 配置</h1></div>

    <div class="card">
      <div class="card-title"><span class="material-icons-round">account</span>账号设置</div>
      <div class="form-row">
        <div class="form-group">
          <label>用户名</label>
          <input class="md-input" id="cfg-user" value="${data.username || ''}">
        </div>
        <div class="form-group">
          <label>新密码（留空不修改）</label>
          <input class="md-input" id="cfg-pass" type="password" placeholder="留空则不修改">
        </div>
      </div>
      <div class="switch-row">
        <label>禁止公网访问</label>
        <label class="md-switch">
          <input type="checkbox" id="cfg-no-wan" ${data.notAllowWanAccess ? 'checked' : ''}>
          <span class="slider"></span>
        </label>
      </div>
    </div>

    <div class="card">
      <div class="card-title">
        <span class="material-icons-round">dns</span>DNS 配置
        <div style="flex:1"></div>
        <button class="md-btn md-btn-outlined" id="add-dns-btn" style="padding:6px 16px;font-size:13px;">
          <span class="material-icons-round" style="font-size:16px;">add</span>添加
        </button>
      </div>
      <div class="dns-tabs" id="dns-tabs">${dnsTabs}</div>
      <div id="dns-panel"></div>
    </div>

    <div class="card">
      <div class="card-title"><span class="material-icons-round">webhook</span>Webhook 通知</div>
      <div class="form-row single">
        <div class="form-group">
          <label>Webhook URL</label>
          <input class="md-input" id="cfg-webhook-url" value="${data.webhookUrl || ''}">
        </div>
      </div>
      <div class="form-row single">
        <div class="form-group">
          <label>Request Body</label>
          <textarea class="md-input" id="cfg-webhook-body">${data.webhookRequestBody || ''}</textarea>
        </div>
      </div>
      <div class="form-row single">
        <div class="form-group">
          <label>Headers</label>
          <textarea class="md-input" id="cfg-webhook-headers">${data.webhookHeaders || ''}</textarea>
        </div>
      </div>
      <div class="btn-row">
        <button class="md-btn md-btn-outlined" id="test-webhook-btn">
          <span class="material-icons-round" style="font-size:16px;">send</span>测试
        </button>
      </div>
    </div>

    <div class="btn-row" style="justify-content:flex-end;">
      <button class="md-btn md-btn-filled" id="save-btn">
        <span class="material-icons-round" style="font-size:16px;">save</span>保存配置
      </button>
    </div>
  `;
}

function renderDnsPanel(idx) {
  const conf = ddnsConfig?.dnsConf?.[idx];
  if (!conf) { document.getElementById('dns-panel').innerHTML = ''; return; }

  const providerOptions = DNS_PROVIDERS.map(p =>
    `<option value="${p.value}" ${conf.dnsName === p.value ? 'selected' : ''}>${p.label}</option>`
  ).join('');

  const ipv4Ifaces = (ddnsConfig.ipv4Interfaces || []).map(i =>
    `<option value="${i.name}" ${conf.ipv4NetInterface === i.name ? 'selected' : ''}>${i.name}</option>`
  ).join('');

  const ipv6Ifaces = (ddnsConfig.ipv6Interfaces || []).map(i =>
    `<option value="${i.name}" ${conf.ipv6NetInterface === i.name ? 'selected' : ''}>${i.name}</option>`
  ).join('');

  document.getElementById('dns-panel').innerHTML = `
    <div class="form-row">
      <div class="form-group">
        <label>DNS 服务商</label>
        <select class="md-select" id="dns-name-${idx}">${providerOptions}</select>
      </div>
      <div class="form-group">
        <label>TTL</label>
        <input class="md-input" id="dns-ttl-${idx}" value="${conf.ttl || '600'}">
      </div>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label>ID / Token</label>
        <input class="md-input" id="dns-id-${idx}" value="${conf.dnsId || ''}">
      </div>
      <div class="form-group">
        <label>Secret / Key</label>
        <input class="md-input" id="dns-secret-${idx}" type="password" value="${conf.dnsSecret || ''}">
      </div>
    </div>
    ${conf.dnsExtParam !== undefined ? `
    <div class="form-row single">
      <div class="form-group">
        <label>扩展参数</label>
        <input class="md-input" id="dns-ext-${idx}" value="${conf.dnsExtParam || ''}">
      </div>
    </div>` : ''}

    <div style="margin:16px 0 8px;font-weight:500;font-size:14px;color:var(--md-primary);">IPv4</div>
    <div class="switch-row">
      <label>启用 IPv4</label>
      <label class="md-switch"><input type="checkbox" id="ipv4-enable-${idx}" ${conf.ipv4Enable ? 'checked' : ''}><span class="slider"></span></label>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label>获取方式</label>
        <select class="md-select" id="ipv4-type-${idx}">
          <option value="url" ${conf.ipv4GetType === 'url' ? 'selected' : ''}>接口</option>
          <option value="netInterface" ${conf.ipv4GetType === 'netInterface' ? 'selected' : ''}>网卡</option>
          <option value="cmd" ${conf.ipv4GetType === 'cmd' ? 'selected' : ''}>命令</option>
        </select>
      </div>
      <div class="form-group" id="ipv4-url-group-${idx}">
        <label>URL</label>
        <input class="md-input" id="ipv4-url-${idx}" value="${conf.ipv4Url || ''}">
      </div>
    </div>
    <div class="form-row single">
      <div class="form-group">
        <label>IPv4 域名（每行一个）</label>
        <textarea class="md-input" id="ipv4-domains-${idx}">${conf.ipv4Domains || ''}</textarea>
      </div>
    </div>

    <div style="margin:16px 0 8px;font-weight:500;font-size:14px;color:var(--md-primary);">IPv6</div>
    <div class="switch-row">
      <label>启用 IPv6</label>
      <label class="md-switch"><input type="checkbox" id="ipv6-enable-${idx}" ${conf.ipv6Enable ? 'checked' : ''}><span class="slider"></span></label>
    </div>
    <div class="form-row">
      <div class="form-group">
        <label>获取方式</label>
        <select class="md-select" id="ipv6-type-${idx}">
          <option value="url" ${conf.ipv6GetType === 'url' ? 'selected' : ''}>接口</option>
          <option value="netInterface" ${conf.ipv6GetType === 'netInterface' ? 'selected' : ''}>网卡</option>
          <option value="cmd" ${conf.ipv6GetType === 'cmd' ? 'selected' : ''}>命令</option>
        </select>
      </div>
      <div class="form-group">
        <label>IPv6 正则</label>
        <input class="md-input" id="ipv6-reg-${idx}" value="${conf.ipv6Reg || ''}">
      </div>
    </div>
    <div class="form-row single">
      <div class="form-group">
        <label>IPv6 域名（每行一个）</label>
        <textarea class="md-input" id="ipv6-domains-${idx}">${conf.ipv6Domains || ''}</textarea>
      </div>
    </div>

    <div class="btn-row" style="justify-content:flex-end;">
      <button class="md-btn md-btn-text" style="color:var(--md-error);" onclick="removeDnsConf(${idx})">
        <span class="material-icons-round" style="font-size:16px;">delete</span>删除此配置
      </button>
    </div>
  `;
}

function bindDDNS() {
  // Tab switching
  document.getElementById('dns-tabs')?.addEventListener('click', (e) => {
    const tab = e.target.closest('.dns-tab');
    if (!tab) return;
    activeDnsTab = parseInt(tab.dataset.tab);
    document.querySelectorAll('.dns-tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    renderDnsPanel(activeDnsTab);
  });

  // Add DNS config
  document.getElementById('add-dns-btn')?.addEventListener('click', () => {
    if (!ddnsConfig) return;
    ddnsConfig.dnsConf = ddnsConfig.dnsConf || [];
    ddnsConfig.dnsConf.push({
      name: '', dnsName: 'alidns', dnsId: '', dnsSecret: '', ttl: '600',
      ipv4Enable: false, ipv4GetType: 'url', ipv4Url: '', ipv4Domains: '',
      ipv6Enable: false, ipv6GetType: 'url', ipv6Url: '', ipv6Domains: '',
    });
    // Re-render tabs
    const tabs = document.getElementById('dns-tabs');
    tabs.innerHTML = ddnsConfig.dnsConf.map((_, i) =>
      `<button class="dns-tab ${i === ddnsConfig.dnsConf.length - 1 ? 'active' : ''}" data-tab="${i}">配置 ${i + 1}</button>`
    ).join('');
    activeDnsTab = ddnsConfig.dnsConf.length - 1;
    renderDnsPanel(activeDnsTab);
  });

  // Initial panel
  if (ddnsConfig?.dnsConf?.length > 0) renderDnsPanel(0);

  // Save
  document.getElementById('save-btn')?.addEventListener('click', async () => {
    const payload = {
      username: document.getElementById('cfg-user')?.value || '',
      password: document.getElementById('cfg-pass')?.value || '',
      notAllowWanAccess: document.getElementById('cfg-no-wan')?.checked || false,
      webhookUrl: document.getElementById('cfg-webhook-url')?.value || '',
      webhookRequestBody: document.getElementById('cfg-webhook-body')?.value || '',
      webhookHeaders: document.getElementById('cfg-webhook-headers')?.value || '',
      dnsConf: (ddnsConfig?.dnsConf || []).map((_, i) => ({
        name: '', ttl: document.getElementById(`dns-ttl-${i}`)?.value || '600',
        dnsName: document.getElementById(`dns-name-${i}`)?.value || 'alidns',
        dnsId: document.getElementById(`dns-id-${i}`)?.value || '',
        dnsSecret: document.getElementById(`dns-secret-${i}`)?.value || '',
        dnsExtParam: document.getElementById(`dns-ext-${i}`)?.value || '',
        ipv4Enable: document.getElementById(`ipv4-enable-${i}`)?.checked || false,
        ipv4GetType: document.getElementById(`ipv4-type-${i}`)?.value || 'url',
        ipv4Url: document.getElementById(`ipv4-url-${i}`)?.value || '',
        ipv4NetInterface: '', ipv4Cmd: '',
        ipv4Domains: document.getElementById(`ipv4-domains-${i}`)?.value || '',
        ipv6Enable: document.getElementById(`ipv6-enable-${i}`)?.checked || false,
        ipv6GetType: document.getElementById(`ipv6-type-${i}`)?.value || 'url',
        ipv6Url: document.getElementById(`ipv6-url-${i}`)?.value || '',
        ipv6NetInterface: '', ipv6Cmd: '', ipv6Reg: document.getElementById(`ipv6-reg-${i}`)?.value || '',
        ipv6Domains: document.getElementById(`ipv6-domains-${i}`)?.value || '',
        httpInterface: '',
      })),
    };
    const res = await apiPost('/api/config/save', payload);
    if (res !== null) toast('配置已保存');
  });

  // Test webhook
  document.getElementById('test-webhook-btn')?.addEventListener('click', async () => {
    await apiPost('/api/webhook/test', {
      url: document.getElementById('cfg-webhook-url')?.value || '',
      requestBody: document.getElementById('cfg-webhook-body')?.value || '',
      headers: document.getElementById('cfg-webhook-headers')?.value || '',
    });
    toast('Webhook 测试已发送');
  });
}

function removeDnsConf(idx) {
  if (!ddnsConfig) return;
  ddnsConfig.dnsConf.splice(idx, 1);
  activeDnsTab = Math.min(activeDnsTab, Math.max(0, ddnsConfig.dnsConf.length - 1));
  // Re-render
  render();
}

// --- Logs ---
async function renderLogs() {
  const logs = await apiGet('/api/logs') || [];
  const logHtml = logs.map(l => {
    let cls = '';
    if (l.includes('失败') || l.includes('异常') || l.includes('error')) cls = 'error';
    else if (l.includes('成功')) cls = 'success';
    return `<div class="log-line ${cls}">${escHtml(l)}</div>`;
  }).join('');

  return `
    <div class="top-bar">
      <h1>运行日志</h1>
      <button class="md-btn md-btn-outlined" id="clear-logs-btn" style="padding:6px 16px;font-size:13px;">
        <span class="material-icons-round" style="font-size:16px;">delete_sweep</span>清除
      </button>
    </div>
    <div class="card" style="padding:0;overflow:hidden;">
      <div class="log-viewer" id="log-viewer">${logHtml || '<div style="color:#888;">暂无日志</div>'}</div>
    </div>
  `;
}

function bindLogs() {
  document.getElementById('clear-logs-btn')?.addEventListener('click', async () => {
    await apiPost('/api/logs/clear', {});
    toast('日志已清除');
    render();
  });
}

// --- Utils ---
function escHtml(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}
