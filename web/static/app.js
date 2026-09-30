'use strict';

const app = document.querySelector('#app');
const dialog = document.querySelector('#dialog');
const toast = document.querySelector('#toast');

const words = {
  en: {
    clients:'Clients', network:'Network', obfuscation:'Obfuscation', addClient:'Add client', addInterface:'New interface', running:'Running', stopped:'Stopped', suspended:'Suspended', active:'Active', search:'Search clients', all:'All clients', online:'Recent handshake', name:'Client', state:'State', handshake:'Latest handshake', transfer:'Transfer', select:'Select a client to inspect its connection and configuration.', noClients:'No clients yet', noClientsText:'Add a device and download its AmneziaWG configuration.', backend:'Host backend', start:'Start', stop:'Stop', edit:'Edit client', config:'Configuration & QR', suspend:'Suspend access', activate:'Restore access', remove:'Delete client', address:'Tunnel address', endpoint:'Endpoint', allowed:'Allowed IPs', created:'Created', received:'Received', sent:'Sent', interface:'Interface', port:'Listen port', subnet:'Subnet', mtu:'MTU', dns:'DNS', autostart:'Starts after reboot', yes:'Yes', no:'No', adopted:'Adopted from host', publicKey:'Public key', configPath:'Host config', checkFirewall:'Check firewall', serverConfig:'View server config', deleteServer:'Delete interface', enabled:'Enabled', disabled:'Disabled', loading:'Reading host state…', agentUnavailable:'Host agent unavailable', kernel:'Kernel module', installed:'Installed', loaded:'Loaded', tools:'awg tools', runtime:'VPN runtime', close:'Close', cancel:'Cancel', save:'Save', delete:'Delete', copy:'Copy', download:'Download .conf', clean:'Clean config', full:'Commented config', link:'AmneziaVPN link', advanced:'Client options', applyI:'Use custom I1–I5 packets', autoSuspend:'Suspend at', never:'Never', createTitle:'Create VPN interface', createIntro:'The configuration is written on the host and the interface is owned by host systemd.', serverName:'Display name', endpointHint:'Public IP or hostname; detected automatically when empty.', autoStart:'Enable at host boot', obfEnable:'Enable AWG 3.x obfuscation', generated:'A safe complete AWG 3.x parameter set and header protection key will be generated.', customParams:'Customize generated parameters', stopTitle:'Stop interface?', stopText:'Connected clients will lose VPN access. Docker and the agent remain management-only.', deleteServerTitle:'Delete interface?', deleteServerText:'This explicit action stops the interface, disables its systemd unit, and removes its host config.', deleteClientTitle:'Delete client?', deleteClientText:'The peer is removed live; the interface is not restarted.', suspendTitle:'Suspend access?', suspendText:'The peer is removed live; other connections stay active.', hostSource:'State is read from the host', settingsReadOnly:'Current host-backed configuration', noServers:'No interfaces found', noServersText:'Create a new interface. Existing compatible host configs are discovered automatically.', refreshIP:'Refresh public IP', current:'Current', parameters:'AWG parameters', headerProtection:'Header Protection', contentPadding:'Content Padding', randomTrailers:'Random Trailers', cookies:'Cookies disabled', junk:'Junk packets', signature:'Header signatures', error:'Request failed', copied:'Copied to clipboard', updated:'Changes saved', clientAdded:'Client added without restarting the interface', interfaceCreated:'Interface created on the host', firewallResult:'Firewall check', healthy:'Ready', attention:'Needs attention', unavailableConfig:'Private key unavailable for this host-adopted peer'
  },
  ru: {
    clients:'Клиенты', network:'Сеть', obfuscation:'Обфускация', addClient:'Добавить клиента', addInterface:'Новый интерфейс', running:'Работает', stopped:'Остановлен', suspended:'Приостановлен', active:'Активен', search:'Поиск клиентов', all:'Все клиенты', online:'Недавний handshake', name:'Клиент', state:'Состояние', handshake:'Последний handshake', transfer:'Трафик', select:'Выберите клиента, чтобы увидеть соединение и конфигурацию.', noClients:'Клиентов пока нет', noClientsText:'Добавьте устройство и скачайте его конфигурацию AmneziaWG.', backend:'Host backend', start:'Запустить', stop:'Остановить', edit:'Изменить клиента', config:'Конфигурация и QR', suspend:'Приостановить доступ', activate:'Вернуть доступ', remove:'Удалить клиента', address:'Адрес в туннеле', endpoint:'Endpoint', allowed:'Allowed IPs', created:'Создан', received:'Получено', sent:'Отправлено', interface:'Интерфейс', port:'Порт', subnet:'Подсеть', mtu:'MTU', dns:'DNS', autostart:'Запуск после reboot', yes:'Да', no:'Нет', adopted:'Подключён с host', publicKey:'Публичный ключ', configPath:'Конфиг на host', checkFirewall:'Проверить firewall', serverConfig:'Открыть конфиг сервера', deleteServer:'Удалить интерфейс', enabled:'Включено', disabled:'Выключено', loading:'Читаю состояние host…', agentUnavailable:'Host agent недоступен', kernel:'Kernel module', installed:'Установлен', loaded:'Загружен', tools:'Утилиты awg', runtime:'VPN runtime', close:'Закрыть', cancel:'Отмена', save:'Сохранить', delete:'Удалить', copy:'Копировать', download:'Скачать .conf', clean:'Чистый конфиг', full:'С комментариями', link:'Ссылка AmneziaVPN', advanced:'Параметры клиента', applyI:'Использовать собственные пакеты I1–I5', autoSuspend:'Приостановить в', never:'Никогда', createTitle:'Создать VPN-интерфейс', createIntro:'Конфиг будет записан на host, а интерфейсом будет управлять host systemd.', serverName:'Название', endpointHint:'Публичный IP или домен; если пусто, определяется автоматически.', autoStart:'Включить запуск на host после reboot', obfEnable:'Включить обфускацию AWG 3.x', generated:'Будет создан полный безопасный набор AWG 3.x и ключ Header Protection.', customParams:'Настроить сгенерированные параметры', stopTitle:'Остановить интерфейс?', stopText:'Клиенты потеряют доступ к VPN. Docker и agent остаются только management plane.', deleteServerTitle:'Удалить интерфейс?', deleteServerText:'Это явное действие остановит интерфейс, отключит его systemd unit и удалит host-конфиг.', deleteClientTitle:'Удалить клиента?', deleteClientText:'Peer удаляется на лету; интерфейс не перезапускается.', suspendTitle:'Приостановить доступ?', suspendText:'Peer удаляется на лету; остальные соединения продолжают работать.', hostSource:'Состояние прочитано с host', settingsReadOnly:'Текущая конфигурация на host', noServers:'Интерфейсы не найдены', noServersText:'Создайте интерфейс. Существующие совместимые host-конфиги обнаруживаются автоматически.', refreshIP:'Обновить публичный IP', current:'Текущий', parameters:'Параметры AWG', headerProtection:'Header Protection', contentPadding:'Content Padding', randomTrailers:'Random Trailers', cookies:'Cookies disabled', junk:'Junk-пакеты', signature:'Сигнатуры заголовков', error:'Ошибка запроса', copied:'Скопировано', updated:'Изменения сохранены', clientAdded:'Клиент добавлен без перезапуска интерфейса', interfaceCreated:'Интерфейс создан на host', firewallResult:'Проверка firewall', healthy:'Готов', attention:'Требует внимания', unavailableConfig:'Private key этого host-adopted peer недоступен'
  }
};
Object.assign(words.en, {
  editEndpoint:'Edit endpoint', endpointTitle:'Client endpoint',
  endpointEditHint:'Enter an IPv4 address or DNS hostname without a port. Leave empty to use the detected public IP.',
  endpointWarning:'Existing client installations are not updated automatically. Re-export or edit them after changing this value.',
  endpointMode:'Endpoint mode', automatic:'Automatic', manual:'Manual'
});
Object.assign(words.ru, {
  editEndpoint:'Изменить endpoint', endpointTitle:'Endpoint клиентов',
  endpointEditHint:'Укажите IPv4-адрес или DNS-имя без порта. Оставьте пустым, чтобы использовать автоопределённый публичный IP.',
  endpointWarning:'Уже импортированные клиентские конфиги не обновятся автоматически. После изменения экспортируйте их заново или отредактируйте вручную.',
  endpointMode:'Режим endpoint', automatic:'Автоматический', manual:'Явный'
});

const state = {
  language: localStorage.getItem('awg-docui-language') || (navigator.language.toLowerCase().startsWith('ru') ? 'ru' : 'en'),
  servers: [], system: null, traffic: {client_traffic:{}, server_traffic:{}}, selectedServer: localStorage.getItem('awg-docui-server'), selectedClient: null,
  tab: 'clients', search: '', filter: 'all', loading: true, busy: false, configData: null
};
const t = key => words[state.language][key] || words.en[key] || key;
const esc = value => String(value ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
const enc = value => encodeURIComponent(String(value));

function icon(name) {
  const paths = {
    plus:'<path d="M12 5v14M5 12h14"/>', search:'<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>', activity:'<path d="M3 12h4l2-6 4 12 2-6h6"/>', play:'<path d="m9 6 9 6-9 6z"/>', stop:'<rect x="7" y="7" width="10" height="10" rx="1"/>', edit:'<path d="m4 20 4-1 11-11-3-3L5 16zM14 6l3 3"/>', qr:'<rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><path d="M14 14h3v3h-3zM20 14v3M14 20h3M20 20h1"/>', trash:'<path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6"/>', pause:'<path d="M9 5v14M15 5v14"/>', check:'<path d="m5 12 4 4L19 6"/>', close:'<path d="m6 6 12 12M18 6 6 18"/>', copy:'<rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>', download:'<path d="M12 3v12m0 0 5-5m-5 5-5-5M5 21h14"/>', chevron:'<path d="m9 18 6-6-6-6"/>', server:'<rect x="4" y="4" width="16" height="6" rx="2"/><rect x="4" y="14" width="16" height="6" rx="2"/><path d="M8 7h.01M8 17h.01"/>', refresh:'<path d="M20 6v5h-5M4 18v-5h5M18 9a7 7 0 0 0-12-2L4 11m16 2-2 4a7 7 0 0 1-12-2"/>', external:'<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 13v6a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h6"/>'
  };
  return `<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">${paths[name] || paths.server}</svg>`;
}

async function api(path, options = {}) {
  const init = {...options, headers:{Accept:'application/json', ...(options.body ? {'Content-Type':'application/json'} : {}), ...(options.headers || {})}};
  const response = await fetch(path, init);
  const type = response.headers.get('content-type') || '';
  const body = type.includes('application/json') ? await response.json() : await response.text();
  if (!response.ok) throw new Error((body && body.error) || (typeof body === 'string' && body) || `${response.status} ${response.statusText}`);
  return body;
}

function currentServer() { return state.servers.find(s => s.id === state.selectedServer) || state.servers[0] || null; }
function currentClient() { return currentServer()?.clients?.find(c => c.id === state.selectedClient) || null; }
function clientTraffic(client) { return state.traffic.client_traffic?.[currentServer()?.id]?.[client.id] || {received:'0 B',sent:'0 B',last_handshake:'Never',endpoint:''}; }
function serverTraffic(server) { return state.traffic.server_traffic?.[server.id] || {rx:'0 B',tx:'0 B'}; }
function isRecent(value) { if (!value || /never|никогда/i.test(value)) return false; const match=value.match(/(\d+)\s+(second|minute)/i); return !!match && (match[2].toLowerCase().startsWith('second') || Number(match[1]) < 4); }
function initials(name) { return String(name || 'C').trim().split(/\s+/).slice(0,2).map(v=>v[0]).join('').toUpperCase(); }
function displayTime(value) { if (!value || /never/i.test(value)) return t('never'); return value; }
function createdAt(value) { return value ? new Intl.DateTimeFormat(state.language === 'ru' ? 'ru-RU' : 'en-GB',{dateStyle:'medium'}).format(new Date(value*1000)) : '—'; }
function bool(value) { return value ? t('yes') : t('no'); }
function endpointOf(server) { const host=server.endpoint || server.public_ip || '—'; return host === '—' ? host : `${host}:${server.port}`; }

function notify(message, error=false) {
  toast.textContent = message; toast.className = `toast visible${error?' error':''}`;
  clearTimeout(notify.timer); notify.timer=setTimeout(()=>toast.className='toast',3800);
}
function showError(error) { notify(error?.message || t('error'), true); }
function setBusy(value) { state.busy=value; document.querySelectorAll('button[type=submit],button[data-mutates]').forEach(b=>b.disabled=value); }

async function loadData({quiet=false}={}) {
  if (!quiet) { state.loading=true; render(); }
  try {
    const [servers, system, traffic] = await Promise.all([api('/api/servers'), api('/api/system/status'), api('/api/traffic')]);
    state.servers=Array.isArray(servers)?servers:[]; state.system=system; state.traffic=traffic || state.traffic;
    if (!state.servers.some(s=>s.id===state.selectedServer)) state.selectedServer=state.servers[0]?.id || null;
    localStorage.setItem('awg-docui-server', state.selectedServer || '');
    const srv=currentServer();
    if (!srv?.clients?.some(c=>c.id===state.selectedClient)) state.selectedClient=srv?.clients?.[0]?.id || null;
  } catch (error) {
    if (!quiet) showError(error);
    state.system=state.system || {backend:{host_agent:false,kernel_module_installed:false,kernel_module_loaded:false,awg_available:false,awg_quick_available:false,vpn_runtime:'Kernel',message:error.message}};
  } finally { state.loading=false; render(); }
}

function topbar() {
  const backend=state.system?.backend || {}; const ready=backend.host_agent&&backend.kernel_module_loaded&&backend.awg_available&&backend.awg_quick_available;
  return `<header class="topbar"><div class="brand"><span class="brand-mark">AWG</span><span>AWG DocUI</span></div><nav class="interface-nav" aria-label="VPN interfaces">${state.servers.map(s=>`<button class="interface-tab" data-server="${esc(s.id)}" aria-current="${s.id===state.selectedServer}"><span class="interface-dot ${s.status==='running'?'up':''}"></span>${esc(s.interface)}</button>`).join('')}<button class="interface-tab" data-action="new-server">${icon('plus')}${t('addInterface')}</button></nav><div class="topbar-actions"><button class="health-button" data-action="health"><span class="health-dot ${ready?'ok':'bad'}"></span><span class="health-label">${t('backend')}</span></button><button class="language-button" data-action="language" aria-label="Change language">${state.language==='ru'?'EN':'RU'}</button></div></header>`;
}

function siteFooter() {
  return `<footer class="site-footer"><a href="https://github.com/Bahonio/amneziawg-docui" target="_blank" rel="noreferrer">Source &amp; licenses · AGPL-3.0-or-later</a></footer>`;
}

function heading(server) {
  if (!server) return '';
  const running=server.status==='running';
  return `<div class="page-heading"><div><div class="eyebrow"><span class="status-pill ${running?'':'stopped'}">${running?t('running'):t('stopped')}</span><span>${esc(server.interface)} · ${t('hostSource')}</span></div><h1 class="page-title">${esc(server.name)}</h1><p class="page-subtitle">${esc(endpointOf(server))} · ${esc(server.subnet)}</p></div><div class="heading-actions"><button class="button" data-action="toggle-server" data-mutates>${running?icon('stop')+t('stop'):icon('play')+t('start')}</button><button class="button primary" data-action="add-client">${icon('plus')}${t('addClient')}</button></div></div>`;
}

function metrics(server) {
  const traffic=serverTraffic(server); const peers=server.clients || []; const recent=peers.filter(c=>isRecent(clientTraffic(c).last_handshake)).length;
  return `<section class="metrics"><div class="metric"><span class="metric-label">${t('state')}</span><strong class="metric-value">${server.status==='running'?t('running'):t('stopped')}</strong><div class="metric-meta">systemd · ${esc(server.interface)}</div></div><div class="metric"><span class="metric-label">${t('clients')}</span><strong class="metric-value">${peers.length}</strong><div class="metric-meta">${recent} ${t('online').toLowerCase()}</div></div><div class="metric"><span class="metric-label">${t('received')}</span><strong class="metric-value">${esc(traffic.rx || '0 B')}</strong><div class="metric-meta">${t('current')} ${esc(server.interface)}</div></div><div class="metric"><span class="metric-label">${t('sent')}</span><strong class="metric-value">${esc(traffic.tx || '0 B')}</strong><div class="metric-meta">${t('current')} ${esc(server.interface)}</div></div></section>`;
}

function tabs() { return `<nav class="tabs" aria-label="Interface sections">${['clients','network','obfuscation'].map(tab=>`<button class="tab" data-tab="${tab}" aria-selected="${state.tab===tab}">${t(tab)}</button>`).join('')}</nav>`; }

function filteredClients(server) {
  const query=state.search.trim().toLowerCase();
  return (server.clients||[]).filter(c => (!query || `${c.name} ${c.client_ip} ${c.id}`.toLowerCase().includes(query)) && (state.filter==='all' || (state.filter==='active' ? c.status!=='suspended' : c.status==='suspended')));
}
function clientRow(client) {
  const traffic=clientTraffic(client); const selected=client.id===state.selectedClient;
  return `<tr data-client="${esc(client.id)}" class="${selected?'selected':''}" aria-selected="${selected}"><td><div class="client-cell"><span class="avatar">${esc(initials(client.name))}</span><div><div class="client-name">${esc(client.name)}</div><div class="client-id">${esc(client.id)}</div></div></div></td><td><span class="state ${client.status==='suspended'?'suspended':''}">${client.status==='suspended'?t('suspended'):t('active')}</span></td><td class="handshake">${esc(displayTime(traffic.last_handshake))}</td><td class="traffic">↓ ${esc(traffic.received)} · ↑ ${esc(traffic.sent)}</td><td class="row-arrow">${icon('chevron')}</td></tr>`;
}
function inspector(client) {
  if (!client) return `<aside class="inspector"><div class="inspector-empty"><div><div class="empty-icon">${icon('activity')}</div><p>${t('select')}</p></div></div></aside>`;
  const traffic=clientTraffic(client);
  return `<aside class="inspector"><div class="inspector-head"><span class="avatar">${esc(initials(client.name))}</span><div><h2 class="inspector-title">${esc(client.name)}</h2><div class="inspector-subtitle">${esc(client.client_ip)} · ${esc(client.id)}</div></div><div class="inspector-actions"><button class="icon-button" data-action="edit-client" title="${t('edit')}">${icon('edit')}</button><button class="icon-button" data-action="client-config" title="${t('config')}" ${client.config_available?'':'disabled'}>${icon('qr')}</button></div></div><dl class="detail-list"><div class="detail-row"><dt>${t('state')}</dt><dd><span class="state ${client.status==='suspended'?'suspended':''}">${client.status==='suspended'?t('suspended'):t('active')}</span></dd></div><div class="detail-row"><dt>${t('handshake')}</dt><dd>${esc(displayTime(traffic.last_handshake))}</dd></div><div class="detail-row"><dt>${t('endpoint')}</dt><dd>${esc(traffic.endpoint || '—')}</dd></div><div class="detail-row"><dt>${t('received')}</dt><dd>${esc(traffic.received)}</dd></div><div class="detail-row"><dt>${t('sent')}</dt><dd>${esc(traffic.sent)}</dd></div><div class="detail-row"><dt>${t('allowed')}</dt><dd>${esc(client.allowed_ips || '0.0.0.0/0')}</dd></div><div class="detail-row"><dt>${t('created')}</dt><dd>${esc(createdAt(client.created_at))}</dd></div></dl>${client.config_available?'':`<div class="banner">${t('unavailableConfig')}</div>`}<div class="detail-actions"><button class="button" data-action="client-config" ${client.config_available?'':'disabled'}>${icon('qr')}${t('config')}</button><button class="button ${client.status==='suspended'?'':'danger'}" data-action="toggle-client" data-mutates>${client.status==='suspended'?icon('play')+t('activate'):icon('pause')+t('suspend')}</button><button class="button quiet" data-action="delete-client" data-mutates>${icon('trash')}${t('remove')}</button></div></aside>`;
}
function clientsView(server) {
  const clients=filteredClients(server);
  if (!(server.clients||[]).length) return `<section class="panel empty"><div><div class="empty-icon">${icon('plus')}</div><h2>${t('noClients')}</h2><p>${t('noClientsText')}</p><button class="button primary" data-action="add-client">${icon('plus')}${t('addClient')}</button></div></section>`;
  return `<section class="panel"><div class="toolbar"><label class="search">${icon('search')}<span class="sr-only">${t('search')}</span><input id="client-search" value="${esc(state.search)}" placeholder="${t('search')}"></label><select class="select" id="client-filter" aria-label="${t('state')}"><option value="all" ${state.filter==='all'?'selected':''}>${t('all')}</option><option value="active" ${state.filter==='active'?'selected':''}>${t('active')}</option><option value="suspended" ${state.filter==='suspended'?'selected':''}>${t('suspended')}</option></select><span class="result-count">${clients.length} / ${(server.clients||[]).length}</span></div><div class="client-layout"><div class="table-wrap"><table><colgroup><col style="width:35%"><col style="width:17%"><col style="width:22%"><col style="width:23%"><col style="width:3%"></colgroup><thead><tr><th>${t('name')}</th><th>${t('state')}</th><th>${t('handshake')}</th><th>${t('transfer')}</th><th></th></tr></thead><tbody>${clients.map(clientRow).join('') || `<tr><td colspan="5" class="muted">${t('select')}</td></tr>`}</tbody></table></div>${inspector(currentClient())}</div></section>`;
}
function property(label,value) { return `<div class="property"><span>${esc(label)}</span><strong>${esc(value ?? '—')}</strong></div>`; }
function networkView(server) {
  return `<section class="settings-grid"><article class="settings-card"><h2>${t('network')}</h2><p>${t('settingsReadOnly')}</p><div class="property-grid">${property(t('interface'),server.interface)}${property(t('port'),`${server.port}/udp`)}${property(t('subnet'),server.subnet)}${property(t('address'),server.server_ip)}${property(t('mtu'),server.mtu)}${property(t('dns'),(server.dns||[]).join(', '))}${property(t('endpoint'),endpointOf(server))}${property(t('endpointMode'),server.endpoint?t('manual'):t('automatic'))}${property(t('autostart'),bool(server.auto_start))}</div><div class="settings-actions"><button class="button small" data-action="edit-endpoint">${icon('edit')}${t('editEndpoint')}</button></div></article><article class="settings-card"><h2>${t('hostSource')}</h2><p>${server.adopted_from_host?t('adopted'):t('interfaceCreated')}</p><div class="property-grid">${property(t('configPath'),server.config_path)}${property(t('publicKey'),server.server_public_key)}${property(t('state'),server.status==='running'?t('running'):t('stopped'))}${property(t('clients'),(server.clients||[]).length)}</div><div class="settings-actions"><button class="button small" data-action="server-config">${t('serverConfig')}</button><button class="button small" data-action="firewall">${t('checkFirewall')}</button><button class="button small" data-action="refresh-ip" data-mutates>${icon('refresh')}${t('refreshIP')}</button></div></article><article class="settings-card wide"><h2>${t('deleteServer')}</h2><p>${t('deleteServerText')}</p><button class="button danger" data-action="delete-server" data-mutates>${icon('trash')}${t('deleteServer')}</button></article></section>`;
}
function obfuscationView(server) {
  const p=server.obfuscation_params || {};
  return `<section class="settings-grid"><article class="settings-card wide"><h2>${t('parameters')}</h2><p>AmneziaWG 3.x · ${server.obfuscation_enabled?t('enabled'):t('disabled')}</p>${server.obfuscation_enabled?`<div class="property-grid">${property(t('junk'),`Jc ${p.Jc ?? '—'} · Jmin ${p.Jmin ?? '—'} · Jmax ${p.Jmax ?? '—'}`)}${property(t('signature'),`H1 ${p.H1 ?? '—'} · H2 ${p.H2 ?? '—'} · H3 ${p.H3 ?? '—'} · H4 ${p.H4 ?? '—'}`)}${property('Padding signatures',`S1 ${p.S1 ?? '—'} · S2 ${p.S2 ?? '—'} · S3 ${p.S3 ?? '—'} · S4 ${p.S4 ?? '—'}`)}${property(t('headerProtection'),p.HeaderProtectionKey?t('enabled'):t('disabled'))}${property(t('contentPadding'),p.ContentPaddingAddition || '—')}${property('RekeyAfterTime',p.RekeyAfterTime || '—')}${property('RekeyTimeout',p.RekeyTimeout || '—')}${property('RejectAfterTime',p.RejectAfterTime || '—')}${property('KeepaliveTimeout',p.KeepaliveTimeout || '—')}${property('MaxHandshakeAttempts',p.MaxHandshakeAttempts || '—')}${property('PersistentKeepalive',p.PersistentKeepalive || '25')}${property(t('randomTrailers'),bool(p.RandomTrailers))}${property(t('cookies'),bool(p.DisableCookies))}</div>`:`<div class="banner">${t('disabled')}</div>`}</article></section>`;
}
function emptyPage() { return `<main class="page"><section class="panel empty"><div><div class="empty-icon">${icon('server')}</div><h2>${t('noServers')}</h2><p>${t('noServersText')}</p><button class="button primary" data-action="new-server">${icon('plus')}${t('addInterface')}</button></div></section></main>`; }
function render() {
  if (state.loading && !state.servers.length) { app.innerHTML=`${topbar()}<div class="loading"><div><div class="spinner"></div>${t('loading')}</div></div>${siteFooter()}`; return; }
  const server=currentServer();
  app.innerHTML=`<div class="shell">${topbar()}${server?`<main class="page">${state.system?.backend?.host_agent===false?`<div class="banner error">${esc(state.system.backend.message || t('agentUnavailable'))}</div>`:''}${heading(server)}${metrics(server)}${tabs()}${state.tab==='clients'?clientsView(server):state.tab==='network'?networkView(server):obfuscationView(server)}</main>`:emptyPage()}${siteFooter()}</div>`;
}

function openDialog(content) { dialog.innerHTML=`<button class="dialog-close" data-action="close-dialog" aria-label="${t('close')}">${icon('close')}</button><div class="dialog-body">${content}</div>`; if (!dialog.open) dialog.showModal(); }
function confirmDialog(title,text,action,label,danger=true) { openDialog(`<h2 class="dialog-title" id="dialog-title">${esc(title)}</h2><p class="dialog-intro">${esc(text)}</p><div class="danger-copy">${esc(text)}</div><div class="form-actions"><button class="button quiet" data-action="close-dialog">${t('cancel')}</button><button class="button ${danger?'danger':'primary'}" data-confirm="${action}" data-mutates>${esc(label)}</button></div>`); }
function healthDialog() {
  const b=state.system?.backend || {}; const rows=[[t('kernel')+' · '+t('installed'),b.kernel_module_installed],[t('kernel')+' · '+t('loaded'),b.kernel_module_loaded],['awg',b.awg_available],['awg-quick',b.awg_quick_available],['Host agent',b.host_agent],[t('runtime'),b.vpn_runtime==='Kernel']];
  openDialog(`<h2 class="dialog-title" id="dialog-title">${t('backend')}</h2><p class="dialog-intro">${esc(b.message || t('hostSource'))}</p><div class="health-list">${rows.map(([label,ok])=>`<div class="health-item"><span>${esc(label)}</span><span class="${ok?'ok':'bad'}">${ok?icon('check')+t('healthy'):t('attention')}</span></div>`).join('')}</div>`);
}
function serverForm() {
  openDialog(`<h2 class="dialog-title" id="dialog-title">${t('createTitle')}</h2><p class="dialog-intro">${t('createIntro')}</p><form class="form" data-form="server"><div class="form-grid"><div class="field full"><label for="server-name">${t('serverName')}</label><input id="server-name" name="name" required maxlength="64" placeholder="Office VPN" autofocus></div><div class="field"><label for="server-port">${t('port')}</label><input id="server-port" name="port" type="number" min="1" max="65535" value="54844"></div><div class="field"><label for="server-subnet">${t('subnet')}</label><input id="server-subnet" name="subnet" value="10.66.66.0/24"></div><div class="field"><label for="server-mtu">${t('mtu')}</label><input id="server-mtu" name="mtu" type="number" min="1280" max="1440" value="1420"></div><div class="field"><label for="server-dns">${t('dns')}</label><input id="server-dns" name="dns" value="1.1.1.1, 1.0.0.1"></div><div class="field full"><label for="server-endpoint">${t('endpoint')}</label><input id="server-endpoint" name="endpoint" placeholder="vpn.example.com"><small>${t('endpointHint')}</small></div></div><label class="check"><input type="checkbox" name="auto_start" checked><span>${t('autoStart')}</span></label><label class="check"><input type="checkbox" name="obfuscation" checked><span><strong>${t('obfEnable')}</strong><br><small>${t('generated')}</small></span></label><details class="advanced"><summary>${t('parameters')}</summary><div class="advanced-content"><label class="check"><input type="checkbox" name="custom_obfuscation"><span>${t('customParams')}</span></label><div class="form-grid" style="margin-top:14px">${[['Jc',8],['Jmin',8],['Jmax',80],['S1',32],['S2',32],['S3',32],['S4',32],['H1',1],['H2',2],['H3',3],['H4',4]].map(([key,value])=>`<div class="field"><label for="op-${key}">${key}</label><input id="op-${key}" name="op_${key}" type="number" min="0" value="${value}"></div>`).join('')}<div class="field full"><label for="op-header">HeaderProtectionKey</label><input id="op-header" name="op_HeaderProtectionKey" placeholder="Generated when empty"></div>${['ContentPaddingAddition','RekeyAfterTime','RekeyTimeout','RejectAfterTime','KeepaliveTimeout','MaxHandshakeAttempts','PersistentKeepalive'].map(key=>`<div class="field"><label for="op-${key}">${key}</label><input id="op-${key}" name="op_${key}" placeholder="integer or a-b"></div>`).join('')}</div><label class="check"><input type="checkbox" name="op_RandomTrailers" checked><span>RandomTrailers</span></label><label class="check"><input type="checkbox" name="op_DisableCookies" checked><span>DisableCookies</span></label></div></details><div class="form-actions"><button type="button" class="button quiet" data-action="close-dialog">${t('cancel')}</button><button class="button primary" type="submit">${t('addInterface')}</button></div></form>`);
}
function endpointForm(server) {
  openDialog(`<h2 class="dialog-title" id="dialog-title">${t('endpointTitle')}</h2><p class="dialog-intro">${esc(endpointOf(server))}</p><form class="form" data-form="endpoint"><div class="field"><label for="edit-server-endpoint">${t('endpoint')}</label><input id="edit-server-endpoint" name="endpoint" value="${esc(server.endpoint||'')}" placeholder="${esc(server.public_ip||'vpn.example.com')}" autofocus><small>${t('endpointEditHint')}</small></div><div class="banner">${t('endpointWarning')}</div><div class="form-actions"><button type="button" class="button quiet" data-action="close-dialog">${t('cancel')}</button><button class="button primary" type="submit">${t('save')}</button></div></form>`);
}
function clientForm(client=null) {
  const i=client?.i_settings || {};
  openDialog(`<h2 class="dialog-title" id="dialog-title">${client?t('edit'):t('addClient')}</h2><p class="dialog-intro">${client?esc(client.client_ip):t('clientAdded')}</p><form class="form" data-form="client" data-client-id="${esc(client?.id||'')}"><div class="field"><label for="client-name">${t('name')}</label><input id="client-name" name="name" required maxlength="64" value="${esc(client?.name||'')}" autofocus></div><div class="field"><label for="allowed-ips">${t('allowed')}</label><input id="allowed-ips" name="allowed_ips" value="${esc(client?.allowed_ips||'0.0.0.0/0')}" required><small>Comma-separated CIDR prefixes</small></div><details class="advanced"><summary>${t('advanced')}</summary><div class="advanced-content"><label class="check"><input type="checkbox" name="apply_i_settings" ${client?.apply_i_settings?'checked':''}><span>${t('applyI')}</span></label><div class="form-grid" style="margin-top:14px">${[1,2,3,4,5].map(n=>`<div class="field ${n===1?'full':''}"><label for="i${n}">I${n}</label><input id="i${n}" name="i${n}" value="${esc(i[`i${n}`]||'')}" placeholder="${n===1?'<b 0x...><r 20>':''}"></div>`).join('')}</div>${client?`<div class="field" style="margin-top:14px"><label for="suspend-at">${t('autoSuspend')}</label><input id="suspend-at" name="suspend_at" type="datetime-local" value="${client.suspend_at?new Date(client.suspend_at*1000).toISOString().slice(0,16):''}"></div>`:''}</div></details><div class="form-actions"><button type="button" class="button quiet" data-action="close-dialog">${t('cancel')}</button><button class="button primary" type="submit">${client?t('save'):t('addClient')}</button></div></form>`);
}
async function configDialog(client) {
  openDialog(`<h2 class="dialog-title" id="dialog-title">${t('config')}</h2><p class="dialog-intro">${esc(client.name)}</p><div class="loading" style="min-height:260px"><div><div class="spinner"></div>${t('loading')}</div></div>`);
  try {
    const server=currentServer(); const [configs,link]=await Promise.all([api(`/api/servers/${enc(server.id)}/clients/${enc(client.id)}/config-both`),api(`/api/servers/${enc(server.id)}/clients/${enc(client.id)}/link`)]); state.configData={configs,link,tab:'qr'}; renderConfigDialog(client);
  } catch(error){dialog.close();showError(error);}
}
function renderConfigDialog(client) {
  const server=currentServer(),d=state.configData,tab=d.tab;
  const tabs=[['qr','QR'],['clean',t('clean')],['full',t('full')],['link',t('link')]];
  let content=tab==='qr'?`<img class="qr" src="/api/servers/${enc(server.id)}/clients/${enc(client.id)}/qr" alt="QR code for ${esc(client.name)}">`:tab==='link'?`<div class="copy-row"><input readonly value="${esc(d.link.vpn_url)}"><button class="button" data-copy="${esc(d.link.vpn_url)}">${icon('copy')}${t('copy')}</button></div>`:`<pre class="code">${esc(tab==='clean'?d.configs.clean_config:d.configs.full_config)}</pre>`;
  dialog.innerHTML=`<button class="dialog-close" data-action="close-dialog" aria-label="${t('close')}">${icon('close')}</button><div class="dialog-body"><h2 class="dialog-title" id="dialog-title">${t('config')}</h2><p class="dialog-intro">${esc(client.name)}</p><div class="code-tabs">${tabs.map(([id,label])=>`<button class="code-tab ${tab===id?'active':''}" data-config-tab="${id}">${label}</button>`).join('')}</div>${content}<div class="form-actions"><a class="button primary" href="/api/servers/${enc(server.id)}/clients/${enc(client.id)}/config" download>${icon('download')}${t('download')}</a><button class="button quiet" data-action="close-dialog">${t('close')}</button></div></div>`;
}
async function serverConfigDialog() {
  try { const s=currentServer(),cfg=await api(`/api/servers/${enc(s.id)}/config`); openDialog(`<h2 class="dialog-title" id="dialog-title">${t('serverConfig')}</h2><p class="dialog-intro">${esc(cfg.config_path)}</p><pre class="code">${esc(cfg.config_content)}</pre><div class="form-actions"><a class="button primary" href="/api/servers/${enc(s.id)}/config/download" download>${icon('download')}${t('download')}</a><button class="button quiet" data-action="close-dialog">${t('close')}</button></div>`); } catch(error){showError(error);}
}
async function firewallDialog() { try { const r=await api(`/api/system/iptables-test?server_id=${enc(currentServer().id)}`); openDialog(`<h2 class="dialog-title" id="dialog-title">${t('firewallResult')}</h2><p class="dialog-intro">${esc(r.interface)} · ${esc(r.subnet)}</p><div class="health-list">${Object.entries(r.iptables_check||{}).map(([k,v])=>`<div class="health-item"><span>${esc(k)}</span><span>${esc(v)}</span></div>`).join('')}</div>`); } catch(error){showError(error);} }

async function mutate(work, success) { if(state.busy)return; setBusy(true); try { await work(); if(dialog.open)dialog.close(); await loadData({quiet:true}); if(success)notify(success); } catch(error){showError(error);} finally {setBusy(false);} }
async function confirmed(action) {
  const s=currentServer(),c=currentClient();
  if(action==='stop-server') await mutate(()=>api(`/api/servers/${enc(s.id)}/stop`,{method:'POST'}),t('updated'));
  if(action==='delete-server') await mutate(()=>api(`/api/servers/${enc(s.id)}`,{method:'DELETE'}),t('updated'));
  if(action==='delete-client') await mutate(()=>api(`/api/servers/${enc(s.id)}/clients/${enc(c.id)}`,{method:'DELETE'}),t('updated'));
  if(action==='suspend-client') await mutate(()=>api(`/api/servers/${enc(s.id)}/clients/${enc(c.id)}/suspend`,{method:'POST'}),t('updated'));
}

app.addEventListener('click', async event => {
  const button=event.target.closest('button'); const row=event.target.closest('[data-client]');
  if(row && !button){state.selectedClient=row.dataset.client;render();return;}
  if(!button)return;
  if(button.dataset.server){state.selectedServer=button.dataset.server;state.selectedClient=currentServer()?.clients?.[0]?.id||null;state.tab='clients';render();return;}
  if(button.dataset.tab){state.tab=button.dataset.tab;render();return;}
  const action=button.dataset.action,s=currentServer(),c=currentClient();
  if(action==='language'){state.language=state.language==='ru'?'en':'ru';localStorage.setItem('awg-docui-language',state.language);document.documentElement.lang=state.language;render();}
  else if(action==='health')healthDialog();
  else if(action==='new-server')serverForm();
  else if(action==='edit-endpoint'&&s)endpointForm(s);
  else if(action==='add-client')clientForm();
  else if(action==='edit-client'&&c)clientForm(c);
  else if(action==='client-config'&&c)configDialog(c);
  else if(action==='toggle-client'&&c){if(c.status==='suspended')await mutate(()=>api(`/api/servers/${enc(s.id)}/clients/${enc(c.id)}/activate`,{method:'POST'}),t('updated'));else confirmDialog(t('suspendTitle'),t('suspendText'),'suspend-client',t('suspend'));}
  else if(action==='delete-client'&&c)confirmDialog(t('deleteClientTitle'),t('deleteClientText'),'delete-client',t('delete'));
  else if(action==='toggle-server'){if(s.status==='running')confirmDialog(t('stopTitle'),t('stopText'),'stop-server',t('stop'));else await mutate(()=>api(`/api/servers/${enc(s.id)}/start`,{method:'POST'}),t('updated'));}
  else if(action==='delete-server')confirmDialog(t('deleteServerTitle'),t('deleteServerText'),'delete-server',t('delete'));
  else if(action==='server-config')serverConfigDialog();
  else if(action==='firewall')firewallDialog();
  else if(action==='refresh-ip')await mutate(()=>api('/api/system/refresh-ip',{method:'POST'}),t('updated'));
});
app.addEventListener('input', event=>{if(event.target.id==='client-search'){state.search=event.target.value;render();document.querySelector('#client-search')?.focus();}});
app.addEventListener('change', event=>{if(event.target.id==='client-filter'){state.filter=event.target.value;render();}});

dialog.addEventListener('click', async event => {
  const button=event.target.closest('button'); if(!button)return;
  if(button.dataset.action==='close-dialog'){dialog.close();return;}
  if(button.dataset.confirm){await confirmed(button.dataset.confirm);return;}
  if(button.dataset.configTab&&state.configData){state.configData.tab=button.dataset.configTab;renderConfigDialog(currentClient());return;}
  if(button.dataset.copy){try{await navigator.clipboard.writeText(button.dataset.copy);notify(t('copied'));}catch(error){showError(error);}}
});
dialog.addEventListener('submit', async event => {
  event.preventDefault(); const form=event.target; const values=new FormData(form); const s=currentServer();
  if(form.dataset.form==='server'){
    const payload={name:values.get('name').trim(),port:Number(values.get('port')),subnet:values.get('subnet').trim(),mtu:Number(values.get('mtu')),endpoint:values.get('endpoint').trim(),dns:values.get('dns').split(',').map(v=>v.trim()).filter(Boolean),auto_start:values.has('auto_start'),obfuscation:values.has('obfuscation')};
    if(payload.obfuscation&&values.has('custom_obfuscation')){payload.obfuscation_params={};for(const key of ['Jc','Jmin','Jmax','S1','S2','S3','S4','H1','H2','H3','H4'])payload.obfuscation_params[key]=Number(values.get(`op_${key}`));for(const key of ['HeaderProtectionKey','ContentPaddingAddition','RekeyAfterTime','RekeyTimeout','RejectAfterTime','KeepaliveTimeout','MaxHandshakeAttempts','PersistentKeepalive'])payload.obfuscation_params[key]=String(values.get(`op_${key}`)||'').trim();payload.obfuscation_params.RandomTrailers=values.has('op_RandomTrailers');payload.obfuscation_params.DisableCookies=values.has('op_DisableCookies');}
    await mutate(async()=>{const made=await api('/api/servers',{method:'POST',body:JSON.stringify(payload)});state.selectedServer=made.id;},t('interfaceCreated'));
  }
  if(form.dataset.form==='endpoint'){
    const payload={endpoint:String(values.get('endpoint')||'').trim()};
    await mutate(()=>api(`/api/servers/${enc(s.id)}/endpoint`,{method:'PUT',body:JSON.stringify(payload)}),t('updated'));
  }
  if(form.dataset.form==='client'){
    const iSettings=Object.fromEntries([1,2,3,4,5].map(n=>[`i${n}`,String(values.get(`i${n}`)||'').trim()]));
    const payload={name:values.get('name').trim(),allowed_ips:values.get('allowed_ips').trim(),apply_i_settings:values.has('apply_i_settings'),i_settings:iSettings}; const id=form.dataset.clientId;
    await mutate(async()=>{if(id){await api(`/api/servers/${enc(s.id)}/clients/${enc(id)}`,{method:'PUT',body:JSON.stringify(payload)});const at=values.get('suspend_at');await api(`/api/servers/${enc(s.id)}/clients/${enc(id)}/suspend-time`,{method:'PUT',body:JSON.stringify({suspend_at:at?new Date(at).toISOString():null})});}else{const made=await api(`/api/servers/${enc(s.id)}/clients`,{method:'POST',body:JSON.stringify(payload)});state.selectedClient=made.client.id;}},id?t('updated'):t('clientAdded'));
  }
});
dialog.addEventListener('close',()=>{state.configData=null;});

document.documentElement.lang=state.language;
loadData();
setInterval(()=>loadData({quiet:true}),5000);
