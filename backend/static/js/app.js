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
    appContainer: document.getElementById('app-container'),
    notFoundContainer: document.getElementById('page-not-found'),
    errorPath: document.getElementById('error-path'),
    homeBtn: document.getElementById('homeBtn'),
};

// Check if current path is valid (only root path is valid for this SPA)
function isValidPath() {
    const path = window.location.pathname;
    // Only root path is valid
    return path === '/' || path === '';
}

// Show 404 page for invalid paths
function show404Page() {
    const currentPath = window.location.pathname;
    elements.appContainer.style.display = 'none';
    elements.notFoundContainer.style.display = 'block';

    // Display the attempted path
    if (elements.errorPath) {
        elements.errorPath.textContent = `Attempted path: ${currentPath || '/'}`;
    }

    console.log('404: Invalid path accessed:', currentPath);
}

// Show main app page
function showMainApp() {
    elements.appContainer.style.display = 'block';
    elements.notFoundContainer.style.display = 'none';
}

// Handle home button click
function handleHomeClick() {
    window.history.pushState(null, '', '/');
    showMainApp();
    // Reload the app
    location.href = '/';
}

// Handle browser back/forward navigation
window.addEventListener('popstate', () => {
    if (!isValidPath()) {
        show404Page();
    } else {
        showMainApp();
    }
});

// Initialize application
document.addEventListener('DOMContentLoaded', () => {
    console.log('Initializing Clicker app...');

    // Check if the current path is valid
    if (!isValidPath()) {
        show404Page();

        // Setup home button listener
        if (elements.homeBtn) {
            elements.homeBtn.addEventListener('click', handleHomeClick);
        }
        return;
    }

    showMainApp();
    setupEventListeners();
    connectWebSocket();
    // loadInitialCounts will be called after receiving auth token from WebSocket
});

// Setup event listeners
function setupEventListeners() {
    elements.clickBtn.addEventListener('click', handleClick);
}

// Handle click event
function handleClick() {
    if (state.isClicking) return;

    if (!state.isWSConnected) {
        updateStatus('Waiting for connection...', 'error', 3000);
        return;
    }

    state.isClicking = true;
    elements.clickBtn.disabled = true;

    try {
        // Send click message via WebSocket
        window.ws.send(JSON.stringify({
            type: 'click'
        }));

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

// Load initial counts via WebSocket
function loadInitialCounts() {
    if (!state.isWSConnected) {
        console.log('Waiting for WebSocket connection before loading counts...');
        setTimeout(loadInitialCounts, 500);
        return;
    }

    if (!window.ws) {
        console.log('WebSocket not ready yet...');
        setTimeout(loadInitialCounts, 500);
        return;
    }

    try {
        // Request count via WebSocket
        window.ws.send(JSON.stringify({
            type: 'get_count'
        }));
        state.isConnected = true;
        updateConnectionStatus();
    } catch (error) {
        console.error('Failed to request counts:', error);
        updateStatus('Failed to load data. Retrying...', 'error', 0);
        setTimeout(loadInitialCounts, 3000);
    }
}

// WebSocket connection
function connectWebSocket() {
    const wsURL = `${CONFIG.WS_PROTOCOL}//${CONFIG.BACKEND_URL.split('//')[1]}/ws`;

    try {
        const ws = new WebSocket(wsURL);
        window.ws = ws; // Store WebSocket globally for message sending

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
                    // Initial counter data will be sent automatically by the server
                    state.isConnected = true;
                    updateConnectionStatus();
                    return;
                }

                // Handle count response
                if (data.type === 'count_response') {
                    state.globalCount = data.global || state.globalCount;
                    state.countries = data.countries || state.countries;
                    updateCounterDisplay();
                    updateLeaderboard();
                    return;
                }

                // Handle countries response
                if (data.type === 'countries_response') {
                    state.countries = data.countries || state.countries;
                    updateLeaderboard();
                    return;
                }

                // Handle broadcast counter updates
                if (data.type === 'counter_update') {
                    state.globalCount = data.global || state.globalCount;
                    state.countries = data.countries || state.countries;
                    updateCounterDisplay();
                    updateLeaderboard();
                    return;
                }

                // Handle click success
                if (data.type === 'click_success') {
                    console.log('Click processed successfully');
                    return;
                }

                // Handle click error
                if (data.type === 'click_error') {
                    const error = data.error || 'Click failed';
                    console.warn('Click error:', error);
                    if (error === 'rate limit exceeded') {
                        updateStatus('Too many clicks! Slow down.', 'error', 3000);
                    }
                    return;
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
            state.isConnected = false;
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

// Periodic sync - refresh counts every 30 seconds if connected
setInterval(() => {
    if (state.isWSConnected && window.ws) {
        window.ws.send(JSON.stringify({
            type: 'get_count'
        }));
    }
}, 30000); // Every 30 seconds
