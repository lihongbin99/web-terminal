(function () {
    const TOKEN_KEY = 'web-terminal-token';

    // Check existing session
    const savedToken = localStorage.getItem(TOKEN_KEY);
    if (savedToken) {
        showTerminal(savedToken);
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
            showTerminal(data.token);
        } catch (err) {
            errorEl.textContent = 'Connection error';
            btn.disabled = false;
        }
    });

    function showTerminal(token) {
        document.getElementById('login-container').style.display = 'none';
        document.getElementById('terminal-container').style.display = 'block';
        initTerminal(token);
    }

    function initTerminal(token) {
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
        const wsUrl = `${protocol}//${location.host}/ws/terminal?token=${encodeURIComponent(token)}&cols=${term.cols}&rows=${term.rows}`;
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
            term.write('\r\n\x1b[31m[Connection closed]\x1b[0m\r\n');
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
    }
})();
