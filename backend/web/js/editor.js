import { api } from './api.js';

const params = new URLSearchParams(window.location.search);
const id = params.get('id');
if (!id) {
    window.location.href = '/';
}

const titleEl = document.getElementById('doc-title');
const exportBtn = document.getElementById('export-btn');
const exportMenu = document.getElementById('export-menu');

function getOrCreate(key, factory) {
    let v = localStorage.getItem(key);
    if (!v) {
        v = factory();
        localStorage.setItem(key, v);
    }
    return v;
}

const userId = getOrCreate('le_user_id', () => (crypto.randomUUID ? crypto.randomUUID() : String(Math.random()).slice(2)));
const userName = getOrCreate('le_user_name', () => 'Пользователь ' + Math.floor(1000 + Math.random() * 9000));

function loadScript(src) {
    return new Promise((resolve, reject) => {
        const s = document.createElement('script');
        s.src = src;
        s.onload = resolve;
        s.onerror = () => reject(new Error('failed to load ' + src));
        document.head.appendChild(s);
    });
}

let currentDoc = null;
let editor = null;
let internalApiUrl = '';

async function init() {
    let cfg;
    try {
        cfg = await api.config();
        internalApiUrl = cfg.internalApiUrl || window.location.origin;
    } catch (err) {
        alert('Не удалось получить конфигурацию: ' + err.message);
        return;
    }

    let data;
    try {
        data = await api.get(id, userId, userName);
    } catch (err) {
        alert('Не удалось открыть документ: ' + err.message);
        return;
    }

    currentDoc = data.document;
    titleEl.textContent = currentDoc.title;
    document.title = `LegalEdit · ${currentDoc.title}`;

    try {
        await loadScript(`${cfg.onlyofficeUrl}/web-apps/apps/api/documents/api.js`);
    } catch (err) {
        alert('Не удалось загрузить OnlyOffice: ' + err.message);
        return;
    }

    const config = data.editorConfig;
    config.events = config.events || {};
    config.events.onRequestHistory = () => refreshHistory();
    config.events.onRequestHistoryData = (event) => loadHistoryVersion(event.data);
    config.events.onRequestHistoryClose = () => editor.refreshHistory(undefined);

    editor = new DocsAPI.DocEditor('editor-placeholder', config);
}

async function refreshHistory() {
    if (!currentDoc) return;
    try {
        const versions = await api.versions(currentDoc.id);
        editor.refreshHistory({
            currentVersion: currentDoc.version,
            history: versions.map((v) => ({
                version: v.version,
                key: `${currentDoc.id}_${v.version}`,
                created: new Date(v.created_at).toLocaleString('ru-RU'),
                user: { id: 'system', name: 'LegalEdit' },
            })),
        });
    } catch (err) {
        console.error('refresh history', err);
    }
}

function loadHistoryVersion(data) {
    if (!currentDoc) return;
    const version = typeof data === 'object' && data !== null ? data.version : data;
    if (!version) return;
    const ext = (currentDoc.filename.split('.').pop() || 'docx').toLowerCase();
    editor.setHistoryData({
        version,
        key: `${currentDoc.id}_${version}`,
        url: `${internalApiUrl}/api/documents/${currentDoc.id}/versions/${version}/file`,
        fileType: ext,
    });
}

titleEl.addEventListener('click', async () => {
    if (!currentDoc) return;
    const next = prompt('Новое название', currentDoc.title);
    if (!next || next.trim() === '' || next === currentDoc.title) return;
    try {
        const updated = await api.rename(currentDoc.id, next.trim());
        currentDoc = updated;
        titleEl.textContent = updated.title;
        document.title = `LegalEdit · ${updated.title}`;
    } catch (err) {
        alert(err.message);
    }
});

exportBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    exportMenu.classList.toggle('hidden');
});

document.addEventListener('click', () => exportMenu.classList.add('hidden'));

exportMenu.querySelectorAll('button').forEach((b) => {
    b.addEventListener('click', () => {
        const format = b.dataset.format;
        exportMenu.classList.add('hidden');
        window.location.href = api.exportUrl(id, format);
    });
});

init();
