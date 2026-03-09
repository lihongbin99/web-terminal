(function () {
    const TOKEN_KEY = 'web-terminal-token';

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
        document.getElementById('login-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'none';
        document.getElementById('dir-container').style.display = 'flex';
        loadDirHistory(token);

        // Open button
        document.getElementById('dir-go-btn').onclick = () => {
            const dir = document.getElementById('dir-input').value.trim();
            if (dir) {
                openTerminal(token, dir);
            }
        };

        // Enter key in input
        document.getElementById('dir-input').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                const dir = document.getElementById('dir-input').value.trim();
                if (dir) {
                    openTerminal(token, dir);
                }
            }
        });

        // Default directory button
        document.getElementById('dir-default-btn').onclick = () => {
            openTerminal(token, '');
        };
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

            dirs.forEach((dir) => {
                const item = document.createElement('div');
                item.className = 'dir-item';
                item.textContent = dir;
                item.onclick = () => openTerminal(token, dir);
                listEl.appendChild(item);
            });
        } catch (err) {
            console.error('Failed to load dir history:', err);
        }
    }

    function openTerminal(token, workDir) {
        document.getElementById('dir-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'block';
        initTerminal(token, workDir);
    }

    function initTerminal(token, workDir) {
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

        const container = document.getElementById('terminal');
        term.open(container);
        fitAddon.fit();

        // WebSocket connection
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        let wsUrl = `${protocol}//${location.host}/ws/terminal?token=${encodeURIComponent(token)}&cols=${term.cols}&rows=${term.rows}`;
        if (workDir) {
            wsUrl += `&workDir=${encodeURIComponent(workDir)}`;
        }
        const ws = new WebSocket(wsUrl);
        ws.binaryType = 'arraybuffer';

        ws.onopen = () => {
            term.focus();
        };

        ws.onmessage = (event) => {
            const data = typeof event.data === 'string' ? event.data : new TextDecoder().decode(event.data);
            term.write(data);
        };

        ws.onclose = () => {
            term.write('\r\n\x1b[31m[Connection closed. Press any key to reconnect...]\x1b[0m\r\n');
            term.onData(() => {
                location.reload();
            });
        };

        ws.onerror = () => {
            term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n');
            localStorage.removeItem(TOKEN_KEY);
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
})();
