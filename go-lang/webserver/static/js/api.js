/**
 * API Client for Axis & Allies Web Interface
 * Handles all communication with the backend REST API
 */

const API_BASE = '/api';

class GameAPI {
    constructor() {
        this.sessionId = null;
    }

    /**
     * Create a new game session
     */
    async createGame(gdfPath, playerName) {
        const response = await fetch(`${API_BASE}/game/new`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                gdfPath,
                playerName
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to create game');
        }

        const data = await response.json();
        this.sessionId = data.sessionId;
        return data;
    }

    /**
     * Get current game state
     */
    async getGameState() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}`);

        if (!response.ok) {
            throw new Error('Failed to get game state');
        }

        return await response.json();
    }

    /**
     * Get all territories
     */
    async getTerritories() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/territories`);

        if (!response.ok) {
            throw new Error('Failed to get territories');
        }

        const data = await response.json();
        return data.territories;
    }

    /**
     * Get the map geometry for this game's board.
     *
     * Served by the API rather than fetched as a static file: the board is
     * chosen at runtime, and the server has already checked that this layout
     * describes that board. Fetched once per game, never polled.
     */
    async getLayout() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/layout`);

        if (!response.ok) {
            throw new Error('Failed to get map layout');
        }
        return await response.json();
    }

    /**
     * Get detailed information about a specific territory
     */
    async getTerritoryDetails(territoryName) {
        this._ensureSession();
        const encodedName = encodeURIComponent(territoryName);
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/territory/${encodedName}`);

        if (!response.ok) {
            throw new Error(`Failed to get territory details for ${territoryName}`);
        }

        return await response.json();
    }

    /**
     * Get available actions for current phase
     */
    async getAvailableActions() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/available-actions`);

        if (!response.ok) {
            throw new Error('Failed to get available actions');
        }

        return await response.json();
    }

    /**
     * Purchase units
     */
    async purchaseUnit(unitType, quantity = 1) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/purchase`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                unitType,
                quantity
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to purchase unit');
        }

        return await response.json();
    }

    /**
     * Plan a move for a unit
     */
    async planMove(pieceId, from, to) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/plan-move`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                pieceId,
                from,
                to
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to plan move');
        }

        return await response.json();
    }

    /**
     * Cancel a planned move
     */
    async cancelMove(pieceId) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/cancel-move`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                pieceId
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to cancel move');
        }

        return await response.json();
    }

    /**
     * Get reachable territories for a piece
     */
    async getReachableTerritories(pieceId, fromTerritory) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/get-reachable`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                pieceId,
                fromTerritory
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to get reachable territories');
        }

        return await response.json();
    }

    /**
     * Mobilize (place) units
     */
    async mobilizeUnits(unitType, territory, quantity = 1) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/mobilize`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                unitType,
                territory,
                quantity
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to mobilize units');
        }

        return await response.json();
    }

    /**
     * Resolve a specific battle
     */
    async resolveBattle(territory) {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/resolve-battle`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                territory
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to resolve battle');
        }

        return await response.json();
    }

    /**
     * Auto-resolve all pending battles
     */
    async autoResolveBattles() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/auto-resolve-battles`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to auto-resolve battles');
        }

        return await response.json();
    }

    /**
     * Advance to next phase
     */
    async advancePhase() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/advance-phase`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to advance phase');
        }

        return await response.json();
    }

    /**
     * Execute NPC turn
     */
    async executeNPCTurn() {
        this._ensureSession();
        const response = await fetch(`${API_BASE}/game/${this.sessionId}/action/execute-npc-turn`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to execute NPC turn');
        }

        return await response.json();
    }

    /**
     * End game session
     */
    async endGame() {
        if (!this.sessionId) return;

        const response = await fetch(`${API_BASE}/game/${this.sessionId}`, {
            method: 'DELETE'
        });

        this.sessionId = null;

        if (!response.ok) {
            throw new Error('Failed to end game session');
        }
    }

    /**
     * Ensure session ID is set
     */
    _ensureSession() {
        if (!this.sessionId) {
            throw new Error('No active game session');
        }
    }
}

// Export as global for use in app.js
window.GameAPI = GameAPI;
