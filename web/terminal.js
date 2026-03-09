(function () {
    const TOKEN_KEY = 'web-terminal-token';

    let currentToken = null;
    let currentSessionId = null;
    let currentWs = null;
    let currentTerm = null;
    let currentFitAddon = null;

    // Check existing session
    const savedToken = localStorage.getItem(TOKEN_KEY);
    if (savedToken) {
        showDirSelection(savedToken);
    }

    // Login form handler
    document.getElementById('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const btn = document.getElementById('login-btn');
        const errorEl = document.getElementById('login-error');
        errorEl.textContent = '';
        btn.disabled = true;

        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;

        try {
            const resp = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password }),
            });
            const data = await resp.json();

            if (!resp.ok) {
                errorEl.textContent = data.error || 'Login failed';
                btn.disabled = false;
                return;
            }

            localStorage.setItem(TOKEN_KEY, data.token);
            showDirSelection(data.token);
        } catch (err) {
            errorEl.textContent = 'Connection error';
            btn.disabled = false;
        }
    });

    function showDirSelection(token) {
        currentToken = token;
        document.getElementById('login-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'none';
        document.getElementById('dir-container').style.display = 'flex';
        loadSessions(token);
        loadDirHistory(token);

        // Open button — create new session
        document.getElementById('dir-go-btn').onclick = () => {
            const dir = document.getElementById('dir-input').value.trim();
            if (dir) {
                createSessionAndConnect(token, dir);
            }
        };

        // Enter key in input
        document.getElementById('dir-input').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                const dir = document.getElementById('dir-input').value.trim();
                if (dir) {
                    createSessionAndConnect(token, dir);
                }
            }
        });

        // Default directory button
        document.getElementById('dir-default-btn').onclick = () => {
            createSessionAndConnect(token, '');
        };

        // Browse button
        let browsePath = '';
        const browserEl = document.getElementById('dir-browser');
        const browserListEl = document.getElementById('dir-browser-list');
        const breadcrumbEl = document.getElementById('dir-breadcrumb');

        document.getElementById('dir-browse-btn').onclick = () => {
            browsePath = '';
            browserEl.style.display = 'block';
            loadBrowseDir(token);
        };

        document.getElementById('dir-close-btn').onclick = () => {
            browserEl.style.display = 'none';
        };

        document.getElementById('dir-confirm-btn').onclick = () => {
            if (browsePath) {
                document.getElementById('dir-input').value = browsePath;
            }
            browserEl.style.display = 'none';
        };

        async function loadBrowseDir(tkn) {
            browserListEl.innerHTML = '';
            updateBreadcrumb();

            try {
                const url = `/api/browse?token=${encodeURIComponent(tkn)}&path=${encodeURIComponent(browsePath)}`;
                const resp = await fetch(url);
                if (!resp.ok) return;
                const data = await resp.json();
                const items = data.items || [];

                items.forEach((name) => {
                    const item = document.createElement('div');
                    item.className = 'dir-browser-item';
                    item.textContent = name;
                    item.onclick = () => {
                        if (browsePath === '') {
                            browsePath = name;
                        } else {
                            browsePath = browsePath + (browsePath.endsWith('\\') ? '' : '\\') + name;
                        }
                        loadBrowseDir(tkn);
                    };
                    browserListEl.appendChild(item);
                });

                if (items.length === 0) {
                    const empty = document.createElement('div');
                    empty.className = 'dir-browser-empty';
                    empty.textContent = 'No subdirectories';
                    browserListEl.appendChild(empty);
                }
            } catch (err) {
                console.error('Failed to browse directory:', err);
            }
        }

        function updateBreadcrumb() {
            breadcrumbEl.innerHTML = '';

            // Root item
            const rootSpan = document.createElement('span');
            rootSpan.className = 'breadcrumb-item';
            rootSpan.textContent = 'Drives';
            rootSpan.onclick = () => {
                browsePath = '';
                loadBrowseDir(token);
            };
            breadcrumbEl.appendChild(rootSpan);

            if (!browsePath) return;

            // Split path into segments
            const parts = browsePath.split('\\').filter(Boolean);
            let accumulated = '';
            parts.forEach((part, i) => {
                const sep = document.createElement('span');
                sep.className = 'breadcrumb-sep';
                sep.textContent = ' > ';
                breadcrumbEl.appendChild(sep);

                if (i === 0 && part.endsWith(':')) {
                    accumulated = part + '\\';
                } else {
                    accumulated = accumulated + (accumulated.endsWith('\\') ? '' : '\\') + part;
                }

                const span = document.createElement('span');
                span.className = 'breadcrumb-item';
                span.textContent = part;
                const pathSnapshot = accumulated;
                span.onclick = () => {
                    browsePath = pathSnapshot;
                    loadBrowseDir(token);
                };
                breadcrumbEl.appendChild(span);
            });
        }
    }

    async function loadSessions(token) {
        const listEl = document.getElementById('session-list');
        listEl.innerHTML = '';

        try {
            const resp = await fetch(`/api/sessions?token=${encodeURIComponent(token)}`);
            if (!resp.ok) return;
            const data = await resp.json();
            const sessions = data.sessions || [];

            if (sessions.length === 0) return;

            const header = document.createElement('div');
            header.className = 'session-list-header';
            header.textContent = 'Active Sessions';
            listEl.appendChild(header);

            sessions.forEach((sess) => {
                const item = document.createElement('div');
                item.className = 'session-item';

                const info = document.createElement('div');
                info.className = 'session-item-info';

                const name = document.createElement('span');
                name.className = 'session-item-name';
                name.textContent = sess.name;
                info.appendChild(name);

                const time = document.createElement('span');
                time.className = 'session-item-time';
                time.textContent = new Date(sess.createdAt).toLocaleString();
                info.appendChild(time);

                const actions = document.createElement('div');
                actions.className = 'session-item-actions';

                const connectBtn = document.createElement('button');
                connectBtn.className = 'session-connect-btn';
                connectBtn.textContent = 'Connect';
                connectBtn.onclick = (e) => {
                    e.stopPropagation();
                    connectToSession(token, sess.id);
                };
                actions.appendChild(connectBtn);

                const deleteBtn = document.createElement('button');
                deleteBtn.className = 'session-delete-btn';
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = async (e) => {
                    e.stopPropagation();
                    await fetch(`/api/sessions?token=${encodeURIComponent(token)}&id=${encodeURIComponent(sess.id)}`, {
                        method: 'DELETE',
                    });
                    loadSessions(token);
                };
                actions.appendChild(deleteBtn);

                item.appendChild(info);
                item.appendChild(actions);
                item.onclick = () => connectToSession(token, sess.id);
                listEl.appendChild(item);
            });
        } catch (err) {
            console.error('Failed to load sessions:', err);
        }
    }

    async function loadDirHistory(token) {
        const listEl = document.getElementById('dir-list');
        listEl.innerHTML = '';

        try {
            const resp = await fetch(`/api/dirs?token=${encodeURIComponent(token)}`);
            if (!resp.ok) {
                if (resp.status === 401) {
                    localStorage.removeItem(TOKEN_KEY);
                    location.reload();
                }
                return;
            }
            const data = await resp.json();
            const dirs = data.dirs || [];

            if (dirs.length > 0) {
                const header = document.createElement('div');
                header.className = 'session-list-header';
                header.textContent = 'Recent Directories';
                listEl.appendChild(header);
            }

            dirs.forEach((dir) => {
                const item = document.createElement('div');
                item.className = 'dir-item';
                item.textContent = dir;
                item.onclick = () => createSessionAndConnect(token, dir);
                listEl.appendChild(item);
            });
        } catch (err) {
            console.error('Failed to load dir history:', err);
        }
    }

    async function createSessionAndConnect(token, workDir) {
        try {
            const resp = await fetch(`/api/sessions?token=${encodeURIComponent(token)}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: workDir || 'Default', workDir, cols: 80, rows: 24 }),
            });
            if (!resp.ok) {
                const data = await resp.json();
                alert(data.error || 'Failed to create session');
                return;
            }
            const data = await resp.json();
            connectToSession(token, data.id);
        } catch (err) {
            console.error('Failed to create session:', err);
        }
    }

    function connectToSession(token, sessionId) {
        currentSessionId = sessionId;
        document.getElementById('dir-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'flex';
        initTerminal(token, sessionId);
    }

    function initTerminal(token, sessionId) {
        // Clean up previous terminal if switching sessions
        if (currentWs) {
            currentWs.onclose = null;
            currentWs.close();
            currentWs = null;
        }
        if (currentTerm) {
            currentTerm.dispose();
            currentTerm = null;
        }

        const container = document.getElementById('terminal');
        container.innerHTML = '';

        const term = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Consolas, "Courier New", monospace',
            theme: {
                background: '#0c0c0c',
                foreground: '#cccccc',
                cursor: '#ffffff',
            },
        });

        const fitAddon = new FitAddon.FitAddon();
        const webLinksAddon = new WebLinksAddon.WebLinksAddon();
        term.loadAddon(fitAddon);
        term.loadAddon(webLinksAddon);

        term.open(container);
        fitAddon.fit();
        setTimeout(() => fitAddon.fit(), 50);

        currentTerm = term;
        currentFitAddon = fitAddon;

        // WebSocket connection
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${location.host}/ws/terminal?token=${encodeURIComponent(token)}&sessionId=${encodeURIComponent(sessionId)}`;
        const ws = new WebSocket(wsUrl);
        ws.binaryType = 'arraybuffer';
        currentWs = ws;

        ws.onopen = () => {
            term.focus();
            // Send initial resize
            ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
        };

        // Toolbar shortcut buttons
        const keyMap = {
            'ctrl-c': '\x03',
            'esc': '\x1b',
            'tab': '\t',
            'up': '\x1b[A',
            'down': '\x1b[B',
            'left': '\x1b[D',
            'right': '\x1b[C',
        };
        document.querySelectorAll('#terminal-toolbar button[data-key]').forEach((btn) => {
            btn.onclick = () => {
                const seq = keyMap[btn.dataset.key];
                if (seq && ws.readyState === WebSocket.OPEN) {
                    ws.send(seq);
                }
                term.focus();
            };
        });

        // Sessions button in toolbar
        document.getElementById('sessions-btn').onclick = () => {
            showSessionOverlay(token);
        };

        ws.onmessage = (event) => {
            const data = typeof event.data === 'string' ? event.data : new TextDecoder().decode(event.data);
            term.write(data);
        };

        ws.onclose = () => {
            term.write('\r\n\x1b[33m[Disconnected from session. Session is still running on server.]\x1b[0m\r\n');
            term.write('\x1b[33m[Press any key to reconnect...]\x1b[0m\r\n');
            const disposable = term.onData(() => {
                disposable.dispose();
                initTerminal(token, sessionId);
            });
        };

        ws.onerror = () => {
            term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n');
        };

        // Send user input to server
        term.onData((data) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(data);
            }
        });

        // Handle resize
        window.addEventListener('resize', () => {
            fitAddon.fit();
        });

        term.onResize(({ cols, rows }) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'resize', cols, rows }));
            }
        });

        // Ctrl+Shift+L to logout
        document.addEventListener('keydown', (e) => {
            if (e.ctrlKey && e.shiftKey && e.key === 'L') {
                localStorage.removeItem(TOKEN_KEY);
                location.reload();
            }
        });
    }

    async function showSessionOverlay(token) {
        const overlay = document.getElementById('session-overlay');
        const listEl = document.getElementById('session-overlay-list');
        overlay.style.display = 'flex';
        listEl.innerHTML = '';

        document.getElementById('session-overlay-close').onclick = () => {
            overlay.style.display = 'none';
            if (currentTerm) currentTerm.focus();
        };

        // Close on background click
        overlay.onclick = (e) => {
            if (e.target === overlay) {
                overlay.style.display = 'none';
                if (currentTerm) currentTerm.focus();
            }
        };

        try {
            const resp = await fetch(`/api/sessions?token=${encodeURIComponent(token)}`);
            if (!resp.ok) return;
            const data = await resp.json();
            const sessions = data.sessions || [];

            if (sessions.length === 0) {
                listEl.innerHTML = '<div class="dir-browser-empty">No active sessions</div>';
                return;
            }

            sessions.forEach((sess) => {
                const item = document.createElement('div');
                item.className = 'session-item' + (sess.id === currentSessionId ? ' session-item-active' : '');

                const info = document.createElement('div');
                info.className = 'session-item-info';

                const name = document.createElement('span');
                name.className = 'session-item-name';
                name.textContent = sess.name;
                info.appendChild(name);

                const time = document.createElement('span');
                time.className = 'session-item-time';
                time.textContent = new Date(sess.createdAt).toLocaleString();
                info.appendChild(time);

                if (sess.id === currentSessionId) {
                    const badge = document.createElement('span');
                    badge.className = 'session-item-badge';
                    badge.textContent = 'current';
                    info.appendChild(badge);
                }

                const actions = document.createElement('div');
                actions.className = 'session-item-actions';

                if (sess.id !== currentSessionId) {
                    const switchBtn = document.createElement('button');
                    switchBtn.className = 'session-connect-btn';
                    switchBtn.textContent = 'Switch';
                    switchBtn.onclick = (e) => {
                        e.stopPropagation();
                        overlay.style.display = 'none';
                        initTerminal(token, sess.id);
                        currentSessionId = sess.id;
                    };
                    actions.appendChild(switchBtn);
                }

                const deleteBtn = document.createElement('button');
                deleteBtn.className = 'session-delete-btn';
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = async (e) => {
                    e.stopPropagation();
                    await fetch(`/api/sessions?token=${encodeURIComponent(token)}&id=${encodeURIComponent(sess.id)}`, {
                        method: 'DELETE',
                    });
                    if (sess.id === currentSessionId) {
                        overlay.style.display = 'none';
                        showDirSelection(token);
                        return;
                    }
                    showSessionOverlay(token);
                };
                actions.appendChild(deleteBtn);

                item.appendChild(info);
                item.appendChild(actions);
                if (sess.id !== currentSessionId) {
                    item.onclick = () => {
                        overlay.style.display = 'none';
                        initTerminal(token, sess.id);
                        currentSessionId = sess.id;
                    };
                }
                listEl.appendChild(item);
            });
        } catch (err) {
            console.error('Failed to load sessions:', err);
        }
    }
})();
