async function request(path, options = {}) {
    const res = await fetch(path, options);
    if (res.status === 204) return null;
    const ct = res.headers.get('content-type') || '';
    const isJson = ct.includes('application/json');
    const body = isJson ? await res.json() : await res.text();
    if (!res.ok) {
        const msg = isJson && body && body.error ? body.error : `HTTP ${res.status}`;
        throw new Error(msg);
    }
    return body;
}

export const api = {
    config: () => request('/api/config'),
    list: (q = '', template = false) => {
        const params = new URLSearchParams();
        if (q) params.set('q', q);
        if (template) params.set('template', 'true');
        const qs = params.toString();
        return request('/api/documents' + (qs ? `?${qs}` : ''));
    },
    instantiate: (id, title) => request(`/api/documents/${id}/instantiate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
    }),
    get: (id, userId, userName) => request(`/api/documents/${id}?user_id=${encodeURIComponent(userId)}&user_name=${encodeURIComponent(userName)}`),
    upload: (file, isTemplate = false) => {
        const fd = new FormData();
        fd.append('file', file);
        if (isTemplate) fd.append('is_template', 'true');
        return request('/api/documents', { method: 'POST', body: fd });
    },
    rename: (id, title) => request(`/api/documents/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
    }),
    remove: (id) => request(`/api/documents/${id}`, { method: 'DELETE' }),
    versions: (id) => request(`/api/documents/${id}/versions`),
    versionFile: (id, version) => `/api/documents/${id}/versions/${version}/file`,
    exportUrl: (id, format) => `/api/documents/${id}/export?format=${format}`,
};
