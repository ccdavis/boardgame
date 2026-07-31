/**
 * Main Vue.js Application for Axis & Allies Web Interface
 */

const { createApp } = Vue;

/**
 * Map geometry, held outside Vue's reactivity on purpose.
 *
 * The layout is ~130 territories of coordinate arrays. Putting it in data()
 * would have Vue deep-proxy every ring and re-diff the lot on each 2-second
 * poll, for data that never changes during a game. SVG path strings are built
 * once here at load; only ownership and unit counts stay reactive.
 */
const MAP = {
    byName: {},
    all: [],
    landShapes: [],
    seaShapes: [],
    links: []
};

function ownerClass(owner) {
    return 'owner-' + String(owner || 'neutral').toLowerCase().replace(/\s+/g, '-');
}

/** Build an SVG path from the layout's polygons/rings/points structure. */
function pathFromPolygons(polygons) {
    let d = '';
    for (const rings of polygons) {
        for (const ring of rings) {
            if (!ring.length) continue;
            d += 'M' + ring.map(p => p[0] + ' ' + p[1]).join('L') + 'Z';
        }
    }
    return d;
}

function ingestLayout(layout) {
    MAP.byName = {};
    MAP.all = [];
    MAP.landShapes = [];
    MAP.seaShapes = [];
    MAP.links = layout.links || [];

    for (const [name, t] of Object.entries(layout.territories)) {
        const isSea = t.kind === 'sea';
        const entry = Object.freeze({
            name,
            isSea,
            d: pathFromPolygons(t.polygons),
            labelX: t.label.x,
            labelY: t.label.y,
            r: t.label.r || 0,
            markerX: t.marker ? t.marker.x : t.label.x,
            markerY: t.marker ? t.marker.y : t.label.y
        });
        MAP.byName[name] = entry;
        MAP.all.push(entry);
        (isSea ? MAP.seaShapes : MAP.landShapes).push(entry);
    }
    Object.freeze(MAP.all);
    Object.freeze(MAP.landShapes);
    Object.freeze(MAP.seaShapes);
}

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

            // Map. The geometry itself is deliberately NOT here: ~130 regions of
            // coordinate arrays would be deep-proxied by Vue and re-diffed on
            // every poll. It is held in a frozen module-level object instead
            // (see loadLayout), and only these small reactive bits live here.
            mapReady: false,
            hoveredTerritory: null,
            view: { x: 0, y: 0, w: 1000, h: 600 },
            mapSize: { w: 1000, h: 600 },

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

        viewBoxStr() {
            const v = this.view;
            return `${v.x} ${v.y} ${v.w} ${v.h}`;
        },

        /** Zoom factor relative to the whole map, used to hide labels that no longer fit. */
        zoom() {
            return this.mapSize.w / this.view.w;
        },

        landShapes() { return MAP.landShapes; },
        seaShapes() { return MAP.seaShapes; },

        /**
         * Labels are dropped when the territory is too small to hold the text at
         * the current zoom. `r` is the inscribed-circle radius at the anchor, so
         * this is a real fit test rather than a guess from area.
         */
        visibleLabels() {
            const out = [];
            for (const t of MAP.all) {
                const half = (t.name.length * (t.isSea ? 1.5 : 1.9)) / 2;
                const fits = t.r * this.zoom >= half * 0.55;
                if (!fits && this.selectedTerritory !== t.name && this.hoveredTerritory !== t.name) {
                    continue;
                }
                out.push({
                    name: t.name, x: t.labelX, y: t.labelY,
                    cls: t.isSea ? 'terr-label sea' : 'terr-label land'
                });
            }
            return out;
        },

        unitMarkers() {
            const out = [];
            for (const terr of this.territories) {
                if (!terr.unitCount) continue;
                const geo = MAP.byName[terr.name];
                if (!geo) continue;
                out.push({
                    name: terr.name, x: geo.markerX, y: geo.markerY,
                    r: Math.max(4, Math.min(9, 3 + Math.sqrt(terr.unitCount) * 1.9)),
                    count: terr.unitCount,
                    cls: 'unit-badge ' + ownerClass(terr.owner)
                });
            }
            return out;
        },

        /**
         * Connectors for adjacencies with no shared border. Only drawn for the
         * selected territory, otherwise they are visual noise across the map.
         */
        visibleLinks() {
            if (!this.selectedTerritory) return [];
            const out = [];
            for (const link of MAP.links) {
                const other = link.a === this.selectedTerritory ? link.b
                            : link.b === this.selectedTerritory ? link.a : null;
                if (!other) continue;
                const from = MAP.byName[this.selectedTerritory];
                const to = MAP.byName[other];
                if (!from || !to) continue;
                out.push({
                    key: link.a + '|' + link.b,
                    x1: from.labelX, y1: from.labelY, x2: to.labelX, y2: to.labelY
                });
            }
            return out;
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

                // Geometry before anything renders, so the map appears complete
                // rather than filling in.
                await this.loadLayout();
                await this.$nextTick();

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
         * Get summary of units grouped by type
         * Returns object like { "infantry": 3, "tank": 2, "fighter": 1 }
         */
        getUnitSummary(units) {
            const summary = {};
            for (const unit of units) {
                const type = unit.name;
                summary[type] = (summary[type] || 0) + 1;
            }
            return summary;
        },

        /**
         * Check if territory belongs to the human player
         */
        isMyTerritory(territoryDetails) {
            return territoryDetails && territoryDetails.owner === this.gameState.humanPlayer;
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
        },

        // --- Map -----------------------------------------------------------
        //
        // There is no "build the overlay" step and no "sync the overlay" step.
        // Territories are rendered by v-for from the frozen geometry, and their
        // classes are bound to game state, so ownership, unit counts and
        // selection follow automatically. The two functions that used to create
        // and then re-synchronise DOM circles are gone: keeping a second set of
        // shapes in agreement with the first was the original bug.

        async loadLayout() {
            try {
                const layout = await this.api.getLayout();
                ingestLayout(layout);

                const [, , w, h] = layout.viewBox;
                this.mapSize = { w, h };
                this.view = { x: 0, y: 0, w, h };
                this.mapReady = true;
            } catch (error) {
                this.error = 'Could not load the map: ' + error.message;
                console.error('Failed to load layout:', error);
            }
        },

        territoryClass(shape) {
            const terr = this.territories.find(t => t.name === shape.name);
            const classes = ['terr', shape.isSea ? 'kind-sea' : 'kind-land'];

            if (this.selectedTerritory === shape.name) classes.push('selected');
            if (this.hoveredTerritory === shape.name) classes.push('hovered');
            if (!terr) return classes;

            classes.push(ownerClass(terr.owner));
            if (terr.unitCount > 0) classes.push('has-units');
            if (terr.owner === this.gameState.humanPlayer) {
                classes.push('friendly');
            } else if (terr.owner === 'Neutral') {
                classes.push('neutral');
            } else {
                classes.push('enemy');
            }
            return classes;
        },

        territoryAria(name) {
            const terr = this.territories.find(t => t.name === name);
            if (!terr) return name;
            const units = terr.unitCount === 1 ? '1 unit' : `${terr.unitCount} units`;
            return `${name}, held by ${terr.owner}, ${units}`;
        },

        clampView(view) {
            const maxW = this.mapSize.w;
            const minW = maxW / 8;
            view.w = Math.max(minW, Math.min(maxW, view.w));
            view.h = view.w * (this.mapSize.h / this.mapSize.w);
            view.x = Math.max(0, Math.min(this.mapSize.w - view.w, view.x));
            view.y = Math.max(0, Math.min(this.mapSize.h - view.h, view.y));
            return view;
        },

        zoomBy(factor, originX, originY) {
            const v = this.view;
            const cx = originX !== undefined ? originX : v.x + v.w / 2;
            const cy = originY !== undefined ? originY : v.y + v.h / 2;
            const w = v.w / factor;
            const h = w * (this.mapSize.h / this.mapSize.w);
            // Keep the point under the cursor fixed while scaling around it.
            this.view = this.clampView({
                x: cx - (cx - v.x) * (w / v.w),
                y: cy - (cy - v.y) * (h / v.h),
                w, h
            });
        },

        resetView() {
            this.view = { x: 0, y: 0, w: this.mapSize.w, h: this.mapSize.h };
        },

        zoomToSelected() {
            const geo = MAP.byName[this.selectedTerritory];
            if (!geo) return;
            const w = this.mapSize.w / 4;
            const h = w * (this.mapSize.h / this.mapSize.w);
            this.view = this.clampView({ x: geo.labelX - w / 2, y: geo.labelY - h / 2, w, h });
        },

        /** Convert a pointer event to a position in map coordinates. */
        eventToMap(event) {
            const svg = document.getElementById('gameMap');
            if (!svg) return null;
            const rect = svg.getBoundingClientRect();
            const scale = this.view.w / rect.width;
            return {
                x: this.view.x + (event.clientX - rect.left) * scale,
                y: this.view.y + (event.clientY - rect.top) * scale
            };
        },

        onWheel(event) {
            const at = this.eventToMap(event);
            this.zoomBy(event.deltaY < 0 ? 1.18 : 1 / 1.18, at && at.x, at && at.y);
        },

        onPanStart(event) {
            const start = this.eventToMap(event);
            if (!start) return;
            const origin = { x: this.view.x, y: this.view.y };
            let moved = false;

            const onMove = (moveEvent) => {
                const rect = document.getElementById('gameMap').getBoundingClientRect();
                const scale = this.view.w / rect.width;
                const dx = (moveEvent.clientX - event.clientX) * scale;
                const dy = (moveEvent.clientY - event.clientY) * scale;
                if (Math.abs(dx) > 1 || Math.abs(dy) > 1) moved = true;
                this.view = this.clampView({
                    x: origin.x - dx, y: origin.y - dy,
                    w: this.view.w, h: this.view.h
                });
            };
            const onUp = () => {
                window.removeEventListener('pointermove', onMove);
                window.removeEventListener('pointerup', onUp);
                // A drag must not also register as a click on the territory
                // underneath, or panning would keep changing the selection.
                if (moved) {
                    const swallow = (e) => e.stopPropagation();
                    window.addEventListener('click', swallow, { capture: true, once: true });
                }
            };
            window.addEventListener('pointermove', onMove);
            window.addEventListener('pointerup', onUp);
        },
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
const mountedApp = app.mount('#app');

// Export for testing and debugging
window.vueApp = mountedApp;
