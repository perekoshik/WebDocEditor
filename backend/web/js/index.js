import { api } from './api.js';

const cardsEl = document.getElementById('cards');
const emptyEl = document.getElementById('empty');
const searchEl = document.getElementById('search');
const dropzoneEl = document.getElementById('dropzone');
const fileInput = document.getElementById('file-input');
const newBtn = document.getElementById('new-doc');
const newBtnLabel = document.getElementById('new-doc-label');
const navItems = document.querySelectorAll('.nav-item');
const emptyTitle = emptyEl.querySelector('h2');
const emptyHint = emptyEl.querySelector('p');

let view = 'documents';
let openMenu = null;

function isTemplatesView() {
    return view === 'templates';
}

function applyViewLabels() {
    if (isTemplatesView()) {
        newBtnLabel.textContent = 'Загрузить шаблон';
        emptyTitle.textContent = 'Здесь будут ваши шаблоны';
        emptyHint.textContent = 'Перетащите .docx-шаблон или нажмите «Загрузить шаблон». Из шаблона можно одним кликом создать новый документ.';
    } else {
        newBtnLabel.textContent = 'Новый документ';
        emptyTitle.textContent = 'Перетащите документ сюда';
        emptyHint.textContent = 'Или нажмите «Новый документ». Поддерживаются .docx, .doc, .odt, .rtf, .txt';
    }
}

function formatDate(iso) {
    const d = new Date(iso);
    const now = new Date();
    const diff = Math.floor((now - d) / 1000);
    if (diff < 60) return 'только что';
    if (diff < 3600) return `${Math.floor(diff / 60)} мин назад`;
    if (diff < 86400) return `${Math.floor(diff / 3600)} ч назад`;
    if (diff < 604800) return `${Math.floor(diff / 86400)} дн назад`;
    return d.toLocaleDateString('ru-RU');
}

function fileIcon() {
    return `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
        <polyline points="14 2 14 8 20 8"></polyline>
    </svg>`;
}

function templateIcon() {
    return `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <rect x="3" y="3" width="18" height="18" rx="2"></rect>
        <line x1="3" y1="9" x2="21" y2="9"></line>
        <line x1="9" y1="3" x2="9" y2="21"></line>
    </svg>`;
}

function kebabIcon() {
    return `<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
        <circle cx="12" cy="5" r="1.5"></circle>
        <circle cx="12" cy="12" r="1.5"></circle>
        <circle cx="12" cy="19" r="1.5"></circle>
    </svg>`;
}

function renderCards(docs) {
    cardsEl.innerHTML = '';
    if (!docs.length) {
        emptyEl.classList.remove('hidden');
        return;
    }
    emptyEl.classList.add('hidden');
    const templates = isTemplatesView();
    for (const d of docs) {
        const card = document.createElement('div');
        card.className = 'card';
        card.dataset.id = d.id;
        const icon = templates ? templateIcon() : fileIcon();
        const metaSuffix = templates ? 'шаблон' : `v${d.version}`;
        const primaryAction = templates
            ? '<button data-action="instantiate">Создать документ</button>'
            : '';
        card.innerHTML = `
            <div class="card-icon">${icon}</div>
            <div class="card-title"></div>
            <div class="card-meta">${formatDate(d.updated_at)} · ${metaSuffix}</div>
            <button class="kebab" aria-label="Меню">${kebabIcon()}</button>
            <div class="menu hidden">
                ${primaryAction}
                <button data-action="rename">Переименовать</button>
                <button data-action="delete">Удалить</button>
            </div>
        `;
        card.querySelector('.card-title').textContent = d.title;
        card.addEventListener('click', (e) => {
            if (e.target.closest('.kebab') || e.target.closest('.menu')) return;
            if (templates) {
                handleInstantiate(d);
            } else {
                window.location.href = `/editor.html?id=${d.id}`;
            }
        });
        const kebab = card.querySelector('.kebab');
        const menu = card.querySelector('.menu');
        kebab.addEventListener('click', (e) => {
            e.stopPropagation();
            if (openMenu && openMenu !== menu) openMenu.classList.add('hidden');
            menu.classList.toggle('hidden');
            openMenu = menu.classList.contains('hidden') ? null : menu;
        });
        const instantiateBtn = menu.querySelector('[data-action="instantiate"]');
        if (instantiateBtn) {
            instantiateBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                menu.classList.add('hidden');
                openMenu = null;
                handleInstantiate(d);
            });
        }
        menu.querySelector('[data-action="rename"]').addEventListener('click', async (e) => {
            e.stopPropagation();
            menu.classList.add('hidden');
            openMenu = null;
            const next = prompt('Новое название', d.title);
            if (!next || next.trim() === '' || next === d.title) return;
            try {
                await api.rename(d.id, next.trim());
                await refresh();
            } catch (err) { alert(err.message); }
        });
        menu.querySelector('[data-action="delete"]').addEventListener('click', async (e) => {
            e.stopPropagation();
            menu.classList.add('hidden');
            openMenu = null;
            if (!confirm(`Удалить «${d.title}»?`)) return;
            try {
                await api.remove(d.id);
                await refresh();
            } catch (err) { alert(err.message); }
        });
        cardsEl.appendChild(card);
    }
}

async function handleInstantiate(template) {
    const title = prompt('Название нового документа', template.title);
    if (!title || !title.trim()) return;
    try {
        const doc = await api.instantiate(template.id, title.trim());
        window.location.href = `/editor.html?id=${doc.id}`;
    } catch (err) {
        alert(err.message);
    }
}

document.addEventListener('click', () => {
    if (openMenu) { openMenu.classList.add('hidden'); openMenu = null; }
});

async function refresh() {
    try {
        const docs = await api.list(searchEl.value.trim(), isTemplatesView());
        renderCards(docs);
    } catch (err) {
        console.error(err);
    }
}

let searchTimer = null;
searchEl.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(refresh, 300);
});

navItems.forEach((item) => {
    item.addEventListener('click', (e) => {
        e.preventDefault();
        const target = item.dataset.view;
        if (!target || target === view) return;
        view = target;
        navItems.forEach((n) => n.classList.toggle('active', n === item));
        applyViewLabels();
        refresh();
    });
});

newBtn.addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', async () => {
    const file = fileInput.files[0];
    fileInput.value = '';
    if (file) await upload(file);
});

async function upload(file) {
    try {
        await api.upload(file, isTemplatesView());
        await refresh();
    } catch (err) {
        alert(err.message);
    }
}

['dragenter', 'dragover'].forEach((evt) => {
    dropzoneEl.addEventListener(evt, (e) => {
        e.preventDefault();
        dropzoneEl.classList.add('dragging');
    });
});
['dragleave', 'drop'].forEach((evt) => {
    dropzoneEl.addEventListener(evt, (e) => {
        e.preventDefault();
        if (evt === 'dragleave' && e.target !== dropzoneEl) return;
        dropzoneEl.classList.remove('dragging');
    });
});
dropzoneEl.addEventListener('drop', async (e) => {
    const files = Array.from(e.dataTransfer.files || []);
    for (const f of files) await upload(f);
});

applyViewLabels();
refresh();
