/**
 * Main Vue.js Application for Axis & Allies Web Interface
 */

const { createApp } = Vue;

const app = createApp({
    data() {
        return {
            // API client
            api: new GameAPI(),

            // Game state
            gameStarted: false,
            gameState: {
                turn: 1,
                currentPower: '',
                currentPhase: '',
                humanPlayer: '',
                isHumanTurn: false,
                players: [],
                victoryCities: { axis: 0, allies: 0 }
            },

            // Setup
            setup: {
                gdfPath: '../aaa.gdf',
                playerName: 'Germany'
            },

            // Territories
            territories: [],
            selectedTerritory: null,
            territoryDetails: null,
            territoryFilter: '',

            // Actions
            availableUnits: [],
            purchasedUnits: [],
            plannedMoves: [],
            pendingBattles: [],

            // UI state
            error: null,
            showPhaseGuidance: false,
            currentPhaseGuidance: '',

            // Polling for state updates
            pollingInterval: null,
        };
    },

    computed: {
        currentPlayer() {
            return this.gameState.players.find(p => p.name === this.gameState.humanPlayer) || { ipcs: 0 };
        },

        filteredTerritories() {
            if (!this.territoryFilter) {
                return this.territories;
            }
            const filter = this.territoryFilter.toLowerCase();
            return this.territories.filter(t =>
                t.name.toLowerCase().includes(filter) ||
                t.owner.toLowerCase().includes(filter)
            );
        },

        phaseGuidance() {
            const phase = this.gameState.currentPhase;
            const guidance = {
                'Purchase Units': `
                    <p><strong>Buy units</strong> with your IPCs. They'll be placed later in the Mobilize phase.</p>
                    <ul>
                        <li>Click "Buy Units" to see available units and their costs</li>
                        <li>Infantry are cheap (3 IPCs) and useful for defense</li>
                        <li>Tanks (5 IPCs) are mobile and powerful</li>
                        <li>Click "Done Purchasing" when ready to proceed</li>
                    </ul>
                `,
                'Combat Move': `
                    <p><strong>Move units to attack</strong> enemy territories or activate neutral territories.</p>
                    <ul>
                        <li>Select a territory with your units</li>
                        <li>Click "Move" on a unit to see where it can go</li>
                        <li>Click a destination to plan the move</li>
                        <li>Review all moves before executing</li>
                        <li>Click "Execute Moves" when ready</li>
                    </ul>
                `,
                'Conduct Combat': `
                    <p><strong>Resolve battles</strong> in territories you attacked.</p>
                    <ul>
                        <li>Battles use dice rolls - units hit on their attack/defend value or less</li>
                        <li>Click "Auto-Resolve All Battles" to let the computer handle it (recommended)</li>
                        <li>Or resolve battles one at a time by clicking on territories</li>
                    </ul>
                `,
                'Noncombat Move': `
                    <p><strong>Reposition units</strong> that didn't attack.</p>
                    <ul>
                        <li>Move to friendly territories only (attacks are done!)</li>
                        <li>Land aircraft from battles</li>
                        <li>Units that attacked cannot move again</li>
                        <li>Click "Execute Moves" when ready</li>
                    </ul>
                `,
                'Mobilize New Units': `
                    <p><strong>Place purchased units</strong> at territories with industrial complexes.</p>
                    <ul>
                        <li>Select a territory with an industrial complex</li>
                        <li>Click on the units you want to place</li>
                        <li>Limited by territory production value</li>
                        <li>Click "Done Placing" when all units are placed</li>
                    </ul>
                `,
                'Collect Income': `
                    <p><strong>Collect IPCs</strong> from your territories.</p>
                    <ul>
                        <li>You'll gain IPCs equal to the production value of your territories</li>
                        <li>These IPCs will be available next turn</li>
                        <li>Click "Collect Income & End Turn" to finish</li>
                    </ul>
                `
            };

            return guidance[phase] || '<p>No guidance available for this phase.</p>';
        }
    },

    methods: {
        /**
         * Start a new game
         */
        async startGame() {
            try {
                this.error = null;
                const result = await this.api.createGame(this.setup.gdfPath, this.setup.playerName);

                this.gameState = result.game;
                this.gameStarted = true;

                // Load initial data
                await this.loadTerritories();
                await this.updateGameState();

                // Show phase guidance on first turn
                this.showPhaseModal();

                // Start polling for state changes
                this.startPolling();

            } catch (error) {
                this.error = error.message;
                console.error('Failed to start game:', error);
            }
        },

        /**
         * Load all territories
         */
        async loadTerritories() {
            try {
                this.territories = await this.api.getTerritories();
            } catch (error) {
                console.error('Failed to load territories:', error);
            }
        },

        /**
         * Update game state from server
         */
        async updateGameState() {
            try {
                const state = await this.api.getGameState();
                const phaseChanged = state.currentPhase !== this.gameState.currentPhase;

                this.gameState = state;

                // Update available actions
                const actions = await this.api.getAvailableActions();

                if (actions.actions) {
                    if (actions.actions.purchase) {
                        this.availableUnits = actions.actions.purchase.availableUnits || [];
                    }
                    if (actions.actions.purchasedUnits) {
                        this.purchasedUnits = actions.actions.purchasedUnits;
                    }
                    if (actions.actions.plannedMoves) {
                        this.plannedMoves = actions.actions.plannedMoves;
                    }
                    if (actions.actions.pendingBattles) {
                        this.pendingBattles = actions.actions.pendingBattles;
                    }
                }

                // Show phase guidance when phase changes during human turn
                if (phaseChanged && this.gameState.isHumanTurn) {
                    this.showPhaseModal();
                }

                // Reload territory details if one is selected
                if (this.selectedTerritory) {
                    await this.loadTerritoryDetails(this.selectedTerritory);
                }

            } catch (error) {
                console.error('Failed to update game state:', error);
            }
        },

        /**
         * Select a territory
         */
        async selectTerritory(territoryName) {
            this.selectedTerritory = territoryName;
            await this.loadTerritoryDetails(territoryName);
        },

        /**
         * Load detailed territory information
         */
        async loadTerritoryDetails(territoryName) {
            try {
                this.territoryDetails = await this.api.getTerritoryDetails(territoryName);
            } catch (error) {
                console.error('Failed to load territory details:', error);
            }
        },

        /**
         * Check if user can interact with a unit
         */
        canInteractWithUnit(unit) {
            // Can only interact with units during human's turn
            if (!this.gameState.isHumanTurn) return false;

            // Check if unit belongs to human player
            const territory = this.territories.find(t => t.name === this.selectedTerritory);
            return territory && territory.owner === this.gameState.humanPlayer;
        },

        /**
         * Move a unit
         */
        async moveUnit(unit) {
            try {
                // Get reachable territories
                const result = await this.api.getReachableTerritories(unit.id, this.selectedTerritory);

                if (!result.reachable || result.reachable.length === 0) {
                    alert('This unit cannot move to any territories');
                    return;
                }

                // For now, use a simple prompt to select destination
                // TODO: Enhance with modal showing destinations
                const destinations = result.reachable.map(t => t.name).join('\n');
                const destination = prompt(`Move ${unit.name} from ${this.selectedTerritory} to:\n\n${destinations}\n\nEnter territory name:`);

                if (destination) {
                    await this.api.planMove(unit.id, this.selectedTerritory, destination);
                    await this.updateGameState();
                    alert(`Move planned: ${unit.name} will move to ${destination}`);
                }
            } catch (error) {
                alert(`Failed to move unit: ${error.message}`);
            }
        },

        /**
         * Purchase units modal
         */
        openPurchaseModal() {
            const modal = document.getElementById('purchaseModal');
            modal.showModal();
        },

        closePurchaseModal() {
            const modal = document.getElementById('purchaseModal');
            modal.close();
        },

        /**
         * Purchase a unit
         */
        async purchaseUnit(unitType) {
            try {
                await this.api.purchaseUnit(unitType, 1);
                await this.updateGameState();
            } catch (error) {
                alert(`Failed to purchase ${unitType}: ${error.message}`);
            }
        },

        /**
         * Show planned moves
         */
        showPlannedMoves() {
            if (this.plannedMoves.length === 0) {
                alert('No moves planned');
                return;
            }

            const movesList = this.plannedMoves.map(m =>
                `Piece ${m.pieceId}: ${m.from} → ${m.to} (${m.type})`
            ).join('\n');

            alert(`Planned Moves:\n\n${movesList}`);
        },

        /**
         * Auto-resolve all battles
         */
        async autoResolveBattles() {
            if (!confirm('Auto-resolve all battles?')) return;

            try {
                const result = await this.api.autoResolveBattles();

                // Show results
                const summary = result.battles.map(b => {
                    const outcome = b.attackerWins ? '✓ Attacker wins!' : '✗ Defender wins';
                    return `${b.territory}: ${outcome} (${b.rounds} rounds)`;
                }).join('\n');

                alert(`Battles Resolved:\n\n${summary}`);

                await this.updateGameState();
            } catch (error) {
                alert(`Failed to resolve battles: ${error.message}`);
            }
        },

        /**
         * Advance to next phase
         */
        async advancePhase() {
            try {
                const result = await this.api.advancePhase();

                if (result.summary) {
                    // Show summary of what happened
                    let message = result.summary;

                    if (result.battles && result.battles.length > 0) {
                        message += `\n\nBattles created in: ${result.battles.join(', ')}`;
                    }

                    if (result.warnings && result.warnings.length > 0) {
                        message += `\n\nWarnings:\n${result.warnings.join('\n')}`;
                    }

                    alert(message);
                }

                await this.updateGameState();

            } catch (error) {
                alert(`Cannot advance phase: ${error.message}`);
            }
        },

        /**
         * Execute NPC turn
         */
        async executeNPCTurn() {
            if (!confirm(`Watch ${this.gameState.currentPower}'s turn?`)) return;

            try {
                const result = await this.api.executeNPCTurn();
                alert(result.summary);
                await this.updateGameState();
                await this.loadTerritories();
            } catch (error) {
                alert(`NPC turn failed: ${error.message}`);
            }
        },

        /**
         * Show phase guidance modal
         */
        showPhaseModal() {
            this.showPhaseGuidance = true;
        },

        closePhaseGuidance() {
            this.showPhaseGuidance = false;
        },

        /**
         * Start polling for game state updates
         */
        startPolling() {
            // Poll every 2 seconds to check for state changes
            this.pollingInterval = setInterval(() => {
                this.updateGameState();
            }, 2000);
        },

        /**
         * Stop polling
         */
        stopPolling() {
            if (this.pollingInterval) {
                clearInterval(this.pollingInterval);
                this.pollingInterval = null;
            }
        }
    },

    mounted() {
        // Cleanup on page unload
        window.addEventListener('beforeunload', () => {
            this.stopPolling();
            if (this.gameStarted) {
                this.api.endGame().catch(console.error);
            }
        });
    },

    unmounted() {
        this.stopPolling();
    }
});

// Mount the app
app.mount('#app');
