// State
let ws = null;
let currentSessionId = null;

// Navigation
document.querySelectorAll('.nav-links a').forEach(link => {
    link.addEventListener('click', (e) => {
        e.preventDefault();
        const page = link.dataset.page;

        document.querySelectorAll('.nav-links a').forEach(l => l.classList.remove('active'));
        link.classList.add('active');

        document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
        document.getElementById(page).classList.add('active');

        loadPage(page);
    });
});

// Load page data
async function loadPage(page) {
    switch (page) {
        case 'dashboard':
            await loadDashboard();
            break;
        case 'sessions':
            await loadSessions();
            break;
        case 'knowledge':
            await loadKnowledge();
            break;
    }
}

// WebSocket connection
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
        console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        handleWebSocketMessage(data);
    };

    ws.onclose = () => {
        console.log('WebSocket disconnected');
        setTimeout(connectWebSocket, 3000);
    };
}

function handleWebSocketMessage(data) {
    const log = document.getElementById('output-log');

    switch (data.type) {
        case 'log':
            appendLog(data.level, data.message);
            break;
        case 'tool_start':
            appendLog('tool', `Calling tool: ${data.tool}`);
            break;
        case 'tool_result':
            appendLog('info', `Tool result: ${data.result.substring(0, 200)}...`);
            break;
        case 'thinking':
            appendLog('info', `Thinking: ${data.content}`);
            break;
        case 'flag':
            appendLog('flag', `FLAG FOUND: ${data.flag}`);
            showResult(data);
            break;
        case 'complete':
            updateStatus(data.success ? 'success' : 'failed');
            if (!data.success) {
                appendLog('error', `Failed: ${data.error}`);
            }
            enableForm(true);
            break;
        case 'error':
            appendLog('error', `Error: ${data.message}`);
            break;
    }
}

function appendLog(level, message) {
    const log = document.getElementById('output-log');
    const entry = document.createElement('div');
    entry.className = `log-entry ${level}`;
    entry.textContent = `[${new Date().toLocaleTimeString()}] ${message}`;
    log.appendChild(entry);
    log.scrollTop = log.scrollHeight;
}

function updateStatus(status) {
    const badge = document.getElementById('solve-status');
    badge.className = `status-badge status-${status}`;
    badge.textContent = status.charAt(0).toUpperCase() + status.slice(1);
}

function showResult(data) {
    const resultDiv = document.getElementById('solve-result');
    resultDiv.style.display = 'block';

    document.getElementById('result-flag').textContent = data.flag;

    const stats = document.getElementById('result-stats');
    stats.innerHTML = `
        <span>Iterations: ${data.iterations || 0}</span>
        <span>Duration: ${data.duration || '0s'}</span>
    `;
}

function enableForm(enabled) {
    document.getElementById('start-solve').disabled = !enabled;
    document.getElementById('challenge-type').disabled = !enabled;
    document.getElementById('challenge-desc').disabled = !enabled;
    document.getElementById('challenge-target').disabled = !enabled;
    document.getElementById('challenge-files').disabled = !enabled;
}

// Start solving
document.getElementById('start-solve').addEventListener('click', async () => {
    const type = document.getElementById('challenge-type').value;
    const desc = document.getElementById('challenge-desc').value;
    const target = document.getElementById('challenge-target').value;
    const files = document.getElementById('challenge-files').value;

    if (!desc && !target) {
        alert('Please provide a description or target');
        return;
    }

    // Reset UI
    document.getElementById('output-log').innerHTML = '';
    document.getElementById('solve-result').style.display = 'none';
    updateStatus('solving');
    enableForm(false);
    appendLog('info', 'Starting challenge solving...');

    try {
        const res = await fetch('/api/solve', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                challenge_type: type,
                description: desc,
                target: target,
                files: files ? files.split(',').map(f => f.trim()) : []
            })
        });

        const data = await res.json();

        if (data.session_id) {
            currentSessionId = data.session_id;
            appendLog('info', `Session created: #${data.session_id}`);
        }
    } catch (err) {
        appendLog('error', `Failed to start: ${err.message}`);
        updateStatus('failed');
        enableForm(true);
    }
});

// Dashboard
async function loadDashboard() {
    try {
        const res = await fetch('/api/dashboard');
        const data = await res.json();

        document.getElementById('stat-total').textContent = data.stats.total_sessions;

        const rate = data.stats.total_sessions > 0
            ? Math.round(data.stats.success_sessions / data.stats.total_sessions * 100)
            : 0;
        document.getElementById('stat-rate').textContent = rate + '%';

        const duration = data.stats.avg_duration_ms
            ? (data.stats.avg_duration_ms / 1000).toFixed(1) + 's'
            : '0s';
        document.getElementById('stat-duration').textContent = duration;

        const typeStats = document.getElementById('type-stats');
        typeStats.innerHTML = Object.entries(data.stats.by_type || {}).map(([type, count]) => `
            <div class="type-card">
                <div class="type-name">${type}</div>
                <div class="type-count">${count}</div>
            </div>
        `).join('');

        renderSessionList('recent-sessions', data.sessions);
    } catch (err) {
        console.error('Failed to load dashboard:', err);
    }
}

// Sessions
async function loadSessions() {
    try {
        const type = document.getElementById('session-type-filter').value;
        const status = document.getElementById('session-status-filter').value;

        let url = '/api/sessions?';
        if (type) url += `type=${type}&`;
        if (status) url += `status=${status}&`;

        const res = await fetch(url);
        const sessions = await res.json();
        renderSessionList('all-sessions', sessions);
    } catch (err) {
        console.error('Failed to load sessions:', err);
    }
}

function renderSessionList(containerId, sessions) {
    const container = document.getElementById(containerId);
    if (!sessions || sessions.length === 0) {
        container.innerHTML = '<div style="padding:20px;text-align:center;color:#666;">No sessions found</div>';
        return;
    }

    container.innerHTML = sessions.map(s => `
        <div class="session-item" onclick="showSession(${s.id})">
            <div class="session-header">
                <span class="session-title">${s.description || 'Session #' + s.id}</span>
                <span>
                    <span class="session-type type-${s.challenge_type}">${s.challenge_type}</span>
                    <span class="status-badge status-${s.status}">${s.status}</span>
                </span>
            </div>
            <div class="session-meta">
                ${s.target ? 'Target: ' + s.target + ' | ' : ''}
                Iterations: ${s.iterations} |
                ${new Date(s.created_at).toLocaleString()}
            </div>
        </div>
    `).join('');
}

async function showSession(id) {
    try {
        const [sessionRes, messagesRes] = await Promise.all([
            fetch(`/api/sessions/${id}`),
            fetch(`/api/sessions/${id}/messages`)
        ]);

        const session = await sessionRes.json();
        const messages = await messagesRes.json();

        let content = `
            <h2>Session #${session.id}</h2>
            <div class="session-info">
                <p><strong>Type:</strong> ${session.challenge_type}</p>
                <p><strong>Status:</strong> <span class="status-badge status-${session.status}">${session.status}</span></p>
                ${session.target ? `<p><strong>Target:</strong> ${session.target}</p>` : ''}
                <p><strong>Description:</strong> ${session.description || 'N/A'}</p>
                <p><strong>Iterations:</strong> ${session.iterations}</p>
                <p><strong>Duration:</strong> ${(session.duration_ms / 1000).toFixed(1)}s</p>
            </div>
        `;

        if (session.flag) {
            content += `<div class="flag-box">Flag: ${session.flag}</div>`;
        }

        content += '<h3>Conversation</h3><div class="conversation">';
        messages.forEach(msg => {
            if (msg.role === 'assistant' && msg.content) {
                content += `<div class="msg assistant"><strong>Agent:</strong><br>${escapeHtml(msg.content)}</div>`;
            } else if (msg.role === 'user' && msg.tool_name) {
                content += `<div class="msg tool"><strong>Tool: ${msg.tool_name}</strong><br>
                    <pre>${escapeHtml(msg.tool_input)}</pre>
                    ${msg.content ? '<br><strong>Result:</strong><br><pre>' + escapeHtml(msg.content) + '</pre>' : ''}
                </div>`;
            }
        });
        content += '</div>';

        document.getElementById('detail-content').innerHTML = content;
        document.getElementById('detail-modal').style.display = 'block';
    } catch (err) {
        console.error('Failed to load session:', err);
    }
}

// Knowledge
async function loadKnowledge() {
    try {
        const type = document.getElementById('knowledge-type-filter').value;
        const res = await fetch(`/api/knowledge?type=${type}`);
        const items = await res.json();
        renderKnowledgeList(items);
    } catch (err) {
        console.error('Failed to load knowledge:', err);
    }
}

function renderKnowledgeList(items) {
    const container = document.getElementById('knowledge-list');
    if (!items || items.length === 0) {
        container.innerHTML = '<div style="padding:20px;text-align:center;color:#666;">No knowledge entries found</div>';
        return;
    }

    container.innerHTML = items.map(k => `
        <div class="knowledge-item" onclick="showKnowledge(${k.id})">
            <div class="knowledge-header">
                <span class="knowledge-title">${k.title}</span>
                <span class="knowledge-type">${k.type}</span>
            </div>
            <div class="knowledge-meta">
                ${new Date(k.created_at).toLocaleString()}
            </div>
        </div>
    `).join('');
}

async function showKnowledge(id) {
    try {
        const res = await fetch(`/api/knowledge/${id}`);
        const data = await res.json();

        const html = marked.parse(data.knowledge.content);

        let content = `
            <h2>${data.knowledge.title}</h2>
            <div class="tags">
                ${(data.tags || []).map(t => `<span class="tag">${t.name}</span>`).join('')}
            </div>
            <div class="markdown-body">${html}</div>
        `;

        document.getElementById('detail-content').innerHTML = content;
        document.getElementById('detail-modal').style.display = 'block';

        document.querySelectorAll('#detail-content pre code').forEach(block => {
            hljs.highlightBlock(block);
        });
    } catch (err) {
        console.error('Failed to load knowledge:', err);
    }
}

async function searchKnowledge() {
    const keyword = document.getElementById('knowledge-search').value;
    if (!keyword) {
        await loadKnowledge();
        return;
    }

    try {
        const res = await fetch(`/api/knowledge/search?q=${encodeURIComponent(keyword)}`);
        const items = await res.json();
        renderKnowledgeList(items);
    } catch (err) {
        console.error('Failed to search knowledge:', err);
    }
}

// Filters
document.getElementById('session-type-filter')?.addEventListener('change', loadSessions);
document.getElementById('session-status-filter')?.addEventListener('change', loadSessions);
document.getElementById('knowledge-type-filter')?.addEventListener('change', loadKnowledge);

// Modal close
document.querySelector('.close').addEventListener('click', () => {
    document.getElementById('detail-modal').style.display = 'none';
});

window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal')) {
        document.getElementById('detail-modal').style.display = 'none';
    }
});

// Helper
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Init
connectWebSocket();
loadPage('solve');
