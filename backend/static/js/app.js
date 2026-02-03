function getWSProtocol(backendURL) {
    // If backend URL is HTTPS, use WSS
    if (backendURL && backendURL.startsWith('https://')) {
        return 'wss:';
    }
    // Otherwise use WS (for localhost/http)
    return 'ws:';
}

// Configuration
const CONFIG = {
    // Backend URL - injected via meta tag or auto-detected
    BACKEND_URL: getBackendURL(),
};

// Set WS protocol after BACKEND_URL is initialized
CONFIG.WS_PROTOCOL = getWSProtocol(CONFIG.BACKEND_URL);

function getBackendURL() {
    // Check URL parameter first (?backend=https://...)
    const urlParams = new URLSearchParams(window.location.search);
    const backendParam = urlParams.get('backend');
    if (backendParam) {
        console.log('Using backend URL from URL parameter:', backendParam);
        return backendParam;
    }

    // Check meta tag for backend URL (injected during deployment)
    const meta = document.querySelector('meta[name="backend-url"]');
    if (meta && meta.getAttribute('content') !== 'BACKEND_URL_PLACEHOLDER') {
        console.log('Using backend URL from meta tag:', meta.getAttribute('content'));
        return meta.getAttribute('content');
    }

    // Use current origin (frontend and backend are now served together)
    console.log('Using current origin as backend URL:', window.location.origin);
    return window.location.origin;
}

// Application state
const state = {
    isConnected: false,
    isWSConnected: false,
    globalCount: 0,
    countries: {},
    isClicking: false,
    authToken: null, // Authentication token from WebSocket
};

// DOM elements
const elements = {
    globalCount: document.getElementById('globalCount'),
    clickBtn: document.getElementById('clickBtn'),
    status: document.getElementById('status'),
    leaderboard: document.getElementById('leaderboard'),
    connectionStatus: document.getElementById('connectionStatus'),
    connectedUsers: document.getElementById('connectedUsers'),
};

// Initialize application
document.addEventListener('DOMContentLoaded', () => {
    console.log('Initializing Clicker app...');
    setupEventListeners();
    connectWebSocket();
    // loadInitialCounts will be called after receiving auth token from WebSocket
});

// Setup event listeners
function setupEventListeners() {
    elements.clickBtn.addEventListener('click', handleClick);
}

// Handle click event
async function handleClick() {
    if (state.isClicking) return;

    if (!state.authToken) {
        updateStatus('Not authenticated. Waiting for token...', 'error', 3000);
        return;
    }

    state.isClicking = true;
    elements.clickBtn.disabled = true;

    try {
        const response = await fetch(`${CONFIG.BACKEND_URL}/click?token=${encodeURIComponent(state.authToken)}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        if (!response.ok) {
            throw new Error(`Click failed: ${response.status}`);
        }

        updateStatus('Click sent! 👍', 'success', 2000);

        // Animate the counter
        const display = elements.globalCount.parentElement;
        display.classList.add('pulse');
        setTimeout(() => display.classList.remove('pulse'), 400);

    } catch (error) {
        console.error('Click error:', error);
        updateStatus('Failed to send click. Check connection.', 'error', 3000);
    } finally {
        state.isClicking = false;
        elements.clickBtn.disabled = false;
    }
}

// Load initial counts
async function loadInitialCounts() {
    try {
        if (!state.authToken) {
            console.log('Waiting for auth token before loading counts...');
            setTimeout(loadInitialCounts, 500);
            return;
        }

        const response = await fetch(`${CONFIG.BACKEND_URL}/count?token=${encodeURIComponent(state.authToken)}`);
        if (!response.ok) throw new Error('Failed to load counts');

        const data = await response.json();
        state.globalCount = data.global || 0;
        state.countries = data.countries || {};

        updateCounterDisplay();
        updateLeaderboard();
        state.isConnected = true;
        updateConnectionStatus();
    } catch (error) {
        console.error('Failed to load initial counts:', error);
        updateStatus('Failed to load data. Retrying...', 'error', 0);
        // Retry after 3 seconds
        setTimeout(loadInitialCounts, 3000);
    }
}

// WebSocket connection
function connectWebSocket() {
    const wsURL = `${CONFIG.WS_PROTOCOL}//${CONFIG.BACKEND_URL.split('//')[1]}/ws`;

    try {
        const ws = new WebSocket(wsURL);

        ws.onopen = () => {
            console.log('WebSocket connected');
            state.isWSConnected = true;
            updateConnectionStatus();
            updateStatus('Connected to server ✓', 'success', 2000);
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);

                // Handle auth token from server
                if (data.type === 'auth_token') {
                    state.authToken = data.token;
                    console.log('Received auth token:', data.token.substring(0, 8) + '...');
                    // Load counts once we have the token
                    loadInitialCounts();
                    return;
                }

                if (data.type === 'counter_update') {
                    state.globalCount = data.global || state.globalCount;
                    state.countries = data.countries || state.countries;
                    updateCounterDisplay();
                    updateLeaderboard();
                }
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            state.isWSConnected = false;
            updateConnectionStatus();
        };

        ws.onclose = () => {
            console.log('WebSocket disconnected');
            state.isWSConnected = false;
            state.authToken = null; // Clear token on disconnect
            updateConnectionStatus();
            // Attempt to reconnect after 3 seconds
            setTimeout(connectWebSocket, 3000);
        };

    } catch (error) {
        console.error('Failed to create WebSocket:', error);
        setTimeout(connectWebSocket, 3000);
    }
}

// Update counter display
function updateCounterDisplay() {
    elements.globalCount.textContent = formatNumber(state.globalCount);
}

// Update leaderboard
function updateLeaderboard() {
    if (!state.countries || Object.keys(state.countries).length === 0) {
        elements.leaderboard.innerHTML = '<p>No countries yet. Be the first to click!</p>';
        return;
    }

    // Sort countries by count (descending) and take top 10
    const sortedCountries = Object.entries(state.countries)
        .map(([key, value]) => ({
            key,
            country: value.country || 'Unknown',
            code: extractCountryCode(key),
            count: value.count || 0,
        }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 10);

    elements.leaderboard.innerHTML = sortedCountries
        .map((item, index) => `
            <div class="country-item">
                <div class="country-info">
                    <div class="country-flag">${getCountryEmoji(item.code)}</div>
                    <div class="country-details">
                        <span class="country-name">${item.country}</span>
                        <span class="country-code">${item.code}</span>
                    </div>
                </div>
                <div class="country-count">${formatNumber(item.count)}</div>
            </div>
        `)
        .join('');
}

// Update connection status
function updateConnectionStatus() {
    const isConnected = state.isConnected && state.isWSConnected;
    elements.connectionStatus.textContent = isConnected ? 'Connected' : 'Disconnected';
    elements.connectionStatus.className = isConnected ? 'connected' : 'connecting';
}

// Update status message
function updateStatus(message, type = 'info', duration = 0) {
    elements.status.textContent = message;
    elements.status.className = `status ${type}`;

    if (duration > 0) {
        setTimeout(() => {
            elements.status.textContent = '';
            elements.status.className = 'status';
        }, duration);
    }
}

// Helper functions
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

function extractCountryCode(docID) {
    if (docID && docID.startsWith('country_')) {
        return docID.substring(8);
    }
    return 'XX';
}

function getCountryEmoji(countryCode) {
    // Convert country code to flag emoji
    if (!countryCode || countryCode === 'XX' || countryCode.length !== 2) {
        return '🌍';
    }

    const codePoints = countryCode
        .toUpperCase()
        .split('')
        .map(char => 127397 + char.charCodeAt());

    try {
        return String.fromCodePoint(...codePoints);
    } catch (error) {
        return '🌍';
    }
}

// Periodic sync (backup mechanism)
setInterval(() => {
    if (!state.isWSConnected && state.authToken) {
        loadInitialCounts();
    }
}, 10000); // Every 10 seconds
