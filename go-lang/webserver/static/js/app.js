/**
 * Main Vue.js Application for Axis & Allies Web Interface
 *
 * Interaction model: the map is the controller, not just the display.
 * Each phase highlights the territories where the player can act, in the
 * manner of classic strategy games:
 *
 *   Purchase   -> your factories glow; click one for its production menu
 *   Move       -> territories with your movable units glow; click one,
 *                 check off the pieces to move, then click a glowing
 *                 destination. Clicking anywhere illegal flashes red.
 *   Combat     -> contested territories pulse; click one for the battle
 *                 screen showing both sides, then fight it out
 *   Mobilize   -> factories that can receive units glow; click to place,
 *                 or place everything where it was bought in one click
 *
 * The sidebar still shows information for whatever is clicked; the mode
 * only decides what ELSE a click does.
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
                victoryCities: { axis: 0, allies: 0 },
                gameOver: false,
                winner: ''
            },
            // Set once the victory overlay has been dismissed, so the player
            // can study the final map without the banner in the way.
            victoryDismissed: false,

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
            // The full territory list is occasionally useful, mostly not; it
            // starts rolled up so the map gets the horizontal room.
            showTerritoryList: false,

            // Map. The geometry itself is deliberately NOT here: ~130 regions of
            // coordinate arrays would be deep-proxied by Vue and re-diffed on
            // every poll. It is held in a frozen module-level object instead
            // (see loadLayout), and only these small reactive bits live here.
            mapReady: false,
            hoveredTerritory: null,
            // Victory cities are always present in the board data; whether they
            // are shown -- and whether holding them wins -- is a play-time
            // choice, not a property of the board.
            showVictoryCities: true,
            view: { x: 0, y: 0, w: 1000, h: 600 },
            mapSize: { w: 1000, h: 600 },

            // Actions
            availableUnits: [],
            purchasedUnits: [],
            plannedMoves: [],
            pendingBattles: [],

            // Interaction state machine. 'idle' means clicks select and, on
            // an eligible territory, open the phase's dialog. 'pickDest' means
            // a group of units is waiting for a destination click.
            ui: {
                mode: 'idle',
                source: null,          // territory the picked units move from
                picked: [],            // piece IDs chosen in the unit picker
                pickedLabel: '',       // human summary, e.g. "3 infantry, 1 armor"
                eligibleDest: {},      // name -> ReachableTerritoryDTO
                flash: null            // territory flashing red after a bad click
            },
            flashTimer: null,

            // Unit picker dialog: one row per unit type with a take-count.
            unitPicker: { territory: '', groups: [] },

            // Purchase dialog is bound to the factory that was clicked, and
            // every unit bought there is earmarked for it so Mobilize can be
            // one click. Earmarks are advisory: Mobilize still checks legality.
            purchaseFor: '',
            earmarks: {},              // unitType -> territory bought at

            // Battle screen.
            battleView: { battle: null, result: null, busy: false },

            // Mobilize placement dialog for one clicked factory.
            placePicker: { territory: '' },

            // In-page dialogs. Native alert()/confirm()/prompt() are banned:
            // a browser lets the user suppress them, after which confirm()
            // silently answers "no" and the game wedges.
            notice: { title: '', body: '' },

            // UI state
            error: null,
            showPhaseGuidance: false,

            // Polling for state updates
            pollingInterval: null,
        };
    },

    computed: {
        currentPlayer() {
            return this.gameState.players.find(p => p.name === this.gameState.humanPlayer) || { ipcs: 0 };
        },

        /** Units bought but not yet on the board -- actual units, not groups. */
        unplacedCount() {
            return this.purchasedUnits.reduce((n, group) => n + group.quantity, 0);
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
         * Where can the player act right now? One set per phase, driving the
         * subtle glow on the map. Empty when it is not the human's turn.
         */
        eligibleSources() {
            const out = {};
            if (!this.gameState.isHumanTurn || this.gameState.gameOver) return out;
            const me = this.gameState.humanPlayer;
            switch (this.gameState.currentPhase) {
                case 'Purchase Units':
                    for (const t of this.territories) {
                        if (t.hasFactory && t.owner === me) out[t.name] = true;
                    }
                    break;
                case 'Combat Move':
                case 'Noncombat Move':
                    for (const t of this.territories) {
                        if (t.friendlyUnits > 0) out[t.name] = true;
                    }
                    break;
                case 'Conduct Combat':
                    for (const b of this.pendingBattles) out[b.territory] = true;
                    break;
                case 'Mobilize New Units':
                    for (const group of this.purchasedUnits) {
                        for (const target of (group.targets || [])) out[target] = true;
                    }
                    break;
            }
            return out;
        },

        /**
         * Stroke-only copies of highlighted territories, drawn in an overlay
         * layer ABOVE all the land. Highlighting the territory paths directly
         * does not work: SVG paints in document order, so a territory's glowing
         * border was overpainted by whichever neighbours came later in the
         * layer, leaving only fragments of the glow visible.
         */
        highlightShapes() {
            const out = [];
            const add = (name, cls) => {
                const geo = MAP.byName[name];
                if (geo) out.push({ name, d: geo.d, cls });
            };
            if (this.ui.mode === 'pickDest') {
                for (const [name, dest] of Object.entries(this.ui.eligibleDest)) {
                    add(name, dest.isAttack ? 'hl dest attack'
                        : dest.isBoard ? 'hl dest board'
                        : 'hl dest');
                }
                if (this.ui.source) add(this.ui.source, 'hl source');
            } else {
                for (const name of Object.keys(this.eligibleSources)) {
                    add(name, 'hl eligible');
                }
            }
            return out;
        },

        /**
         * Factory glyphs, drawn for every industrial complex on the map.
         * Positioned beside the unit badge, never under it: both anchor at the
         * territory's marker point, and the badge layer painted over the
         * factory in exactly the territories that matter (a factory town
         * always has a garrison).
         */
        factoryMarkers() {
            const out = [];
            for (const terr of this.territories) {
                if (!terr.hasFactory) continue;
                const geo = MAP.byName[terr.name];
                if (!geo) continue;
                const badgeR = terr.unitCount
                    ? Math.max(4, Math.min(9, 3 + Math.sqrt(terr.unitCount) * 1.9))
                    : 0;
                out.push({
                    name: terr.name,
                    x: geo.markerX - (badgeR ? badgeR + 6.2 : 0),
                    y: geo.markerY,
                    mine: terr.owner === this.gameState.humanPlayer
                });
            }
            return out;
        },

        /** Crossed swords over each territory with a battle waiting. */
        battleMarkers() {
            if (this.gameState.currentPhase !== 'Conduct Combat') return [];
            const out = [];
            for (const b of this.pendingBattles) {
                const geo = MAP.byName[b.territory];
                if (!geo) continue;
                out.push({ name: b.territory, x: geo.markerX, y: geo.markerY });
            }
            return out;
        },

        /** Planned moves drawn as arrows, combat moves in red. */
        moveArrows() {
            const out = [];
            for (const m of this.plannedMoves) {
                const from = MAP.byName[m.from];
                const to = MAP.byName[m.to];
                if (!from || !to) continue;
                const key = m.from + '>' + m.to + ':' + m.type;
                if (out.some(a => a.key === key)) continue; // one arrow per route
                out.push({
                    key,
                    x1: from.labelX, y1: from.labelY,
                    x2: to.labelX, y2: to.labelY,
                    cls: m.type === 'combat' ? 'move-arrow combat' : 'move-arrow'
                });
            }
            return out;
        },

        /** Status line shown in the action bar while a destination is awaited. */
        modeBanner() {
            if (this.ui.mode !== 'pickDest') return '';
            const kinds = [];
            const dests = Object.values(this.ui.eligibleDest);
            if (dests.some(d => d.isAttack)) kinds.push('red = attack');
            if (dests.some(d => d.isBoard)) kinds.push('blue = board transport');
            const legend = kinds.length ? ` (${kinds.join(', ')})` : '';
            return `Moving ${this.ui.pickedLabel} from ${this.ui.source} — ` +
                   `click a highlighted destination${legend}, Esc cancels`;
        },

        /** Every earmarked purchase that is still legal to place as planned. */
        placeAsBoughtPlan() {
            const plan = [];
            for (const group of this.purchasedUnits) {
                const target = this.earmarks[group.type];
                if (!target || !(group.targets || []).includes(target)) return [];
                plan.push({ type: group.type, territory: target, quantity: group.quantity });
            }
            return plan;
        },

        /** Pending groups placeable at the territory the place picker shows. */
        placeableGroups() {
            const here = this.placePicker.territory;
            return this.purchasedUnits.filter(g => (g.targets || []).includes(here));
        },

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

        /**
         * Victory cities, drawn from the polled game state rather than the
         * layout: which territories are victory cities is board data, but a
         * game may switch the condition off.
         */
        victoryMarkers() {
            if (!this.showVictoryCities) return [];
            const out = [];
            for (const terr of this.territories) {
                if (!terr.isVictoryCity) continue;
                const geo = MAP.byName[terr.name];
                if (!geo) continue;
                // Offset above the label so the star and the name do not collide.
                out.push({
                    name: terr.name,
                    x: geo.labelX,
                    y: geo.labelY - Math.max(geo.r * 0.5, 5),
                    r: 4.6
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
                // Badges are coloured by who owns the UNITS, not the ground.
                // For sea zones this is the only sensible colour: the zone is
                // Neutral, but the fleet in it belongs to somebody.
                out.push({
                    name: terr.name, x: geo.markerX, y: geo.markerY,
                    r: Math.max(4, Math.min(9, 3 + Math.sqrt(terr.unitCount) * 1.9)),
                    count: terr.unitCount,
                    cls: 'unit-badge ' + ownerClass(terr.pieceOwner || terr.owner)
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

        /** How many units the unit picker currently has checked. */
        pickerTotal() {
            return this.unitPicker.groups.reduce((n, g) => n + g.take, 0);
        },

        phaseGuidance() {
            const phase = this.gameState.currentPhase;
            const guidance = {
                'Purchase Units': `
                    <p><strong>Buy units</strong> with your IPCs. They arrive in the Mobilize phase.</p>
                    <ul>
                        <li>Your factories glow on the map — click one to open its production menu</li>
                        <li>Units you buy there will be offered there when you place them</li>
                        <li>Infantry (3 IPCs) hold ground; armor (5 IPCs) hits harder and moves 2</li>
                        <li>Click "Done Purchasing" when you're finished</li>
                    </ul>
                `,
                'Combat Move': `
                    <p><strong>Move units to attack.</strong> Territories with your units glow.</p>
                    <ul>
                        <li>Click a glowing territory and check off the units to move</li>
                        <li>Legal destinations light up — red means an attack</li>
                        <li>Click the destination to lock in the move (arrows show your plan)</li>
                        <li>Click "Execute Moves" to march</li>
                    </ul>
                `,
                'Conduct Combat': `
                    <p><strong>Resolve battles</strong> where your attacks landed.</p>
                    <ul>
                        <li>Contested territories are marked with crossed swords — click one</li>
                        <li>The battle screen shows both sides; "Fight it out" rolls the dice</li>
                        <li>Or "Resolve All Battles" to settle everything at once</li>
                    </ul>
                `,
                'Noncombat Move': `
                    <p><strong>Reposition units</strong> that didn't attack.</p>
                    <ul>
                        <li>Same as combat movement, but only friendly destinations light up</li>
                        <li>Land your aircraft somewhere safe — they cannot end in the air</li>
                        <li>Click "Execute Moves" when done</li>
                    </ul>
                `,
                'Mobilize New Units': `
                    <p><strong>Place your purchases.</strong> Factories that can receive units glow.</p>
                    <ul>
                        <li>"Place all as bought" puts everything where you bought it</li>
                        <li>Or click a glowing factory to place units by hand</li>
                        <li>A factory can build up to its territory's production value</li>
                    </ul>
                `,
                'Collect Income': `
                    <p><strong>Collect IPCs</strong> from your territories.</p>
                    <ul>
                        <li>You'll gain IPCs equal to the production value of your territories</li>
                        <li>Click "Collect Income & End Turn" to finish</li>
                    </ul>
                `
            };

            return guidance[phase] || '<p>No guidance available for this phase.</p>';
        }
    },

    methods: {
        /**
         * Show a message in the in-page notice dialog.
         */
        showNotice(title, body) {
            this.notice = { title, body };
            document.getElementById('noticeModal').showModal();
        },

        closeNotice() {
            document.getElementById('noticeModal').close();
        },

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
                await this.updateGameState(true);

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
         * Update game state from server.
         *
         * Two callers, two costs. The 2-second poll calls this bare: one GET,
         * and if nothing it reports has changed (turn, power, phase, verdict),
         * that is the END of it -- no action lists, no territory sweep. The
         * game is turn-based; during the player's own turn the server state
         * only changes through the player's own clicks, so every action
         * handler calls this with force=true to pull the fresh lists it just
         * invalidated. The poll's remaining job is to notice out-of-band
         * changes: another tab acting on the same session, or a future
         * server-driven opponent.
         */
        async updateGameState(force = false) {
            try {
                const state = await this.api.getGameState();
                const prev = this.gameState;
                const phaseChanged = state.currentPhase !== prev.currentPhase;
                const changed = force || phaseChanged ||
                    state.turn !== prev.turn ||
                    state.currentPower !== prev.currentPower ||
                    state.gameOver !== prev.gameOver;

                // The header (turn, phase, IPC counts) always follows the
                // freshest snapshot; that much is one cheap assignment.
                this.gameState = state;

                // The war can end on anyone's turn; the poll is how we hear.
                // Phase guidance is beside the point once there are no more
                // phases, and its modal would sit on top of the verdict.
                if (this.gameState.gameOver) {
                    this.showPhaseGuidance = false;
                    this.stopPolling();
                }

                if (!changed) return; // idle poll: nothing to refresh

                // A phase change invalidates any half-finished interaction:
                // picked units from combat move mean nothing in noncombat move.
                if (phaseChanged) this.cancelTargeting();

                await this.refreshActions();

                // The territory list feeds the sidebar and the map badges;
                // ownership and unit counts can change on any action.
                await this.loadTerritories();

                // Show phase guidance when phase changes during human turn
                if (phaseChanged && this.gameState.isHumanTurn && !this.gameState.gameOver) {
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

        /** Fetch the phase's action data: purchase catalog, planned moves,
         *  pending battles, unplaced units. Called on phase entry and after
         *  the player's own actions -- never from an idle poll. */
        async refreshActions() {
            const actions = await this.api.getAvailableActions();
            if (!actions.actions) return;
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

        // --- Map interaction ------------------------------------------------

        /**
         * The single entry point for a click on the map. What it does depends
         * on the interaction mode and the phase; showing information in the
         * sidebar happens regardless.
         */
        async onTerritoryClick(name) {
            // Destination mode: the click IS the answer.
            if (this.ui.mode === 'pickDest') {
                if (this.ui.eligibleDest[name]) {
                    await this.commitMove(name);
                } else if (name === this.ui.source) {
                    this.cancelTargeting();
                } else {
                    this.flashBad(name);
                }
                return;
            }

            await this.selectTerritory(name);

            if (!this.gameState.isHumanTurn || this.gameState.gameOver) return;
            if (!this.eligibleSources[name]) {
                // Never fail silently: a click that cannot act flashes red,
                // the same signal as an ineligible destination.
                const phase = this.gameState.currentPhase;
                if (phase === 'Combat Move' || phase === 'Noncombat Move') {
                    const terr = this.territories.find(t => t.name === name);
                    if (terr && terr.owner === this.gameState.humanPlayer) {
                        this.flashBad(name);
                    }
                }
                return;
            }

            switch (this.gameState.currentPhase) {
                case 'Purchase Units':
                    this.openPurchaseFor(name);
                    break;
                case 'Combat Move':
                case 'Noncombat Move':
                    await this.openUnitPicker(name);
                    break;
                case 'Conduct Combat':
                    this.openBattle(name);
                    break;
                case 'Mobilize New Units':
                    this.openPlacePicker(name);
                    break;
            }
        },

        /** A brief red flash on a territory that cannot be the answer. */
        flashBad(name) {
            this.ui.flash = name;
            clearTimeout(this.flashTimer);
            this.flashTimer = setTimeout(() => { this.ui.flash = null; }, 450);
        },

        cancelTargeting() {
            this.ui.mode = 'idle';
            this.ui.source = null;
            this.ui.picked = [];
            this.ui.pickedLabel = '';
            this.ui.eligibleDest = {};
        },

        // --- Unit picker ----------------------------------------------------

        /**
         * Open the unit picker for a territory: the player's movable units
         * there, one row per type, all selected to start (moving the whole
         * stack is the common case; trimming it down is the exception).
         */
        async openUnitPicker(territory) {
            let details;
            try {
                details = await this.api.getTerritoryDetails(territory);
            } catch (error) {
                this.showNotice('Cannot inspect territory', error.message);
                return;
            }

            const me = this.gameState.humanPlayer;
            const groups = {};
            for (const unit of (details.units || [])) {
                if (unit.owner !== me || !unit.canMove || unit.movement <= 0) continue;
                // Cargo aboard a transport groups separately from free units of
                // the same type: its destinations are shores, not roads.
                const key = unit.name + (unit.aboard ? '|cargo' : '');
                if (!groups[key]) {
                    groups[key] = {
                        type: unit.name,
                        cargo: !!unit.aboard,
                        attack: unit.attack, defend: unit.defend, movement: unit.movement,
                        ids: [], take: 0
                    };
                }
                groups[key].ids.push(unit.id);
            }

            const list = Object.values(groups).sort((a, b) => a.type.localeCompare(b.type));
            if (list.length === 0) {
                this.showNotice('No movable units',
                    `No units in ${territory} can still move this phase.`);
                return;
            }
            for (const g of list) g.take = g.ids.length;

            this.unitPicker = { territory, groups: list };
            document.getElementById('unitPickerModal').showModal();
        },

        adjustTake(group, delta) {
            group.take = Math.max(0, Math.min(group.ids.length, group.take + delta));
        },

        closeUnitPicker() {
            document.getElementById('unitPickerModal').close();
        },

        /**
         * OK on the unit picker: ask the server where this whole group can go,
         * then switch to destination mode and light those territories up.
         */
        async confirmUnitPicker() {
            const picked = [];
            const labelParts = [];
            let cargoTaken = 0, freeTaken = 0;
            for (const g of this.unitPicker.groups) {
                if (g.take > 0) {
                    picked.push(...g.ids.slice(0, g.take));
                    labelParts.push(`${g.take} ${g.type}${g.cargo ? ' (aboard)' : ''}`);
                    if (g.cargo) cargoTaken += g.take; else freeTaken += g.take;
                }
            }
            if (picked.length === 0) return;
            // Cargo unloads onto shores; ships sail to sea zones. One click
            // cannot answer both, so the two are moved separately.
            if (cargoTaken > 0 && freeTaken > 0) {
                this.showNotice('Move these separately',
                    'Units aboard transports unload onto land, while ships move ' +
                    'between sea zones — pick one group or the other, not both.');
                return;
            }
            this.closeUnitPicker();

            let result;
            try {
                result = await this.api.getReachableForPieces(picked, this.unitPicker.territory);
            } catch (error) {
                this.showNotice('Cannot plan that move', error.message);
                return;
            }

            const dests = {};
            for (const dest of (result.reachable || [])) dests[dest.name] = dest;
            if (Object.keys(dests).length === 0) {
                this.showNotice('Nowhere to go',
                    'No territory is reachable by every unit you selected. ' +
                    'Try a smaller group — slow units limit the fast ones.');
                return;
            }

            this.ui.mode = 'pickDest';
            this.ui.source = this.unitPicker.territory;
            this.ui.picked = picked;
            this.ui.pickedLabel = labelParts.join(', ');
            this.ui.eligibleDest = dests;
        },

        /**
         * Destination clicked. An ordinary territory books one planned move
         * per picked unit; a boarding sea zone loads the group onto transports
         * there; an unload shore disembarks the cargo. Loads and unloads take
         * effect immediately -- they are not planned moves.
         */
        async commitMove(destination) {
            const dest = this.ui.eligibleDest[destination];
            const picked = this.ui.picked.slice();
            const source = this.ui.source;
            this.cancelTargeting();

            try {
                if (dest.isBoard) {
                    await this.api.loadTransports(picked, destination);
                } else if (dest.isUnload) {
                    await this.api.unloadTransport(picked, destination);
                } else {
                    const failures = [];
                    for (const pieceId of picked) {
                        try {
                            await this.api.planMove(pieceId, source, destination);
                        } catch (error) {
                            failures.push(error.message);
                        }
                    }
                    if (failures.length > 0) {
                        this.showNotice(`${failures.length} of ${picked.length} moves refused`,
                            failures.join('\n'));
                    }
                }
            } catch (error) {
                this.showNotice('Could not complete that move', error.message);
            }
            await this.updateGameState(true);
        },

        /** Undo the most recently planned move (or booked landing). */
        async undoLastMove() {
            const last = this.plannedMoves[this.plannedMoves.length - 1];
            if (!last) return;
            await this.cancelPlannedMove(last);
        },

        /** m is a planned-move entry; landings cancel through their own API. */
        async cancelPlannedMove(m) {
            try {
                if (m.landing) {
                    await this.api.cancelLanding(m.pieceId);
                } else {
                    await this.api.cancelMove(m.pieceId);
                }
                await this.updateGameState(true);
            } catch (error) {
                this.showNotice('Cannot cancel move', error.message);
            }
        },

        showPlannedMoves() {
            document.getElementById('movesModal').showModal();
        },

        closePlannedMoves() {
            document.getElementById('movesModal').close();
        },

        // --- Purchasing -----------------------------------------------------

        /** Open the production menu anchored to a clicked factory. */
        async openPurchaseFor(territory) {
            this.purchaseFor = territory;
            document.getElementById('purchaseModal').showModal();
            // The catalog's affordability flags are only as fresh as the last
            // action; re-check on open rather than trusting an idle poll.
            try {
                await this.refreshActions();
            } catch (error) {
                console.error('Failed to refresh purchase list:', error);
            }
        },

        closePurchaseModal() {
            document.getElementById('purchaseModal').close();
        },

        /** Buy one unit and earmark it for the factory whose menu is open. */
        async purchaseUnit(unitType) {
            try {
                const result = await this.api.purchaseUnit(unitType, 1);
                if (this.purchaseFor) {
                    this.earmarks[unitType] = this.purchaseFor;
                }
                if (result.purchasedUnits) this.purchasedUnits = result.purchasedUnits;
                await this.updateGameState(true);
            } catch (error) {
                this.showNotice(`Cannot buy ${unitType}`, error.message);
            }
        },

        // --- Mobilize -------------------------------------------------------

        openPlacePicker(territory) {
            this.placePicker = { territory };
            document.getElementById('placeModal').showModal();
        },

        closePlacePicker() {
            document.getElementById('placeModal').close();
        },

        /**
         * Place purchased units during the Mobilize phase.
         */
        async placeUnits(unitType, territory, quantity) {
            if (!territory) {
                this.showNotice('Nowhere to place', `No legal territory to place ${unitType} in.`);
                return;
            }
            try {
                await this.api.mobilizeUnits(unitType, territory, quantity);
                await this.updateGameState(true);
            } catch (error) {
                this.showNotice(`Cannot place ${unitType} in ${territory}`, error.message);
            }
        },

        /** Place every pending unit at the factory where it was bought. */
        async placeAllAsBought() {
            const plan = this.placeAsBoughtPlan;
            for (const step of plan) {
                try {
                    await this.api.mobilizeUnits(step.type, step.territory, step.quantity);
                } catch (error) {
                    this.showNotice(`Cannot place ${step.type} in ${step.territory}`, error.message);
                    break;
                }
            }
            await this.updateGameState(true);
        },

        // --- Combat ---------------------------------------------------------

        /** Open the battle screen for a contested territory. */
        openBattle(territory) {
            const battle = this.pendingBattles.find(b => b.territory === territory);
            if (!battle) return;
            this.battleView = { battle, result: null, busy: false };
            document.getElementById('battleModal').showModal();
        },

        async closeBattle() {
            document.getElementById('battleModal').close();
            await this.updateGameState(true);
        },

        /**
         * Fight the open battle to its end. For now the dice run without
         * pauses; per-round decisions (retreat, casualty choice) come later.
         */
        async fightBattle() {
            const battle = this.battleView.battle;
            if (!battle || this.battleView.busy) return;
            this.battleView.busy = true;
            try {
                const response = await this.api.resolveBattle(battle.territory);
                this.battleView.result = response.result;
            } catch (error) {
                this.showNotice('Battle failed to resolve', error.message);
            } finally {
                this.battleView.busy = false;
            }
        },

        /** Group casualty names into "2 infantry, 1 armor" for the summary. */
        casualtySummary(names) {
            if (!names || names.length === 0) return 'no losses';
            const counts = {};
            for (const n of names) counts[n] = (counts[n] || 0) + 1;
            return Object.entries(counts).map(([n, c]) => `${c} ${n}`).join(', ');
        },

        /**
         * Auto-resolve all battles
         */
        async autoResolveBattles() {
            try {
                const result = await this.api.autoResolveBattles();

                const summary = result.battles.map(b => {
                    const outcome = b.attackerWins ? '✓ Attacker wins!' : '✗ Defender wins';
                    return `${b.territory}: ${outcome} (${b.rounds} rounds)`;
                }).join('\n');

                this.showNotice('Battles resolved', summary);

                await this.updateGameState(true);
            } catch (error) {
                this.showNotice('Battle resolution failed', error.message);
            }
        },

        /**
         * Advance to next phase
         */
        async advancePhase() {
            this.cancelTargeting();
            try {
                const result = await this.api.advancePhase();

                // The phase has unfinished business; say what, and stay put.
                if (result.blocked) {
                    this.showNotice('Cannot end this phase yet', result.summary);
                    return;
                }

                // Routine transitions pass silently -- the header already
                // announces the new phase. A dialog appears only when there
                // is something worth reading: battles created, or warnings.
                const parts = [];
                if (result.battles && result.battles.length > 0) {
                    parts.push(`Battles created in: ${result.battles.join(', ')}`);
                }
                if (result.warnings && result.warnings.length > 0) {
                    parts.push(`Warnings:\n${result.warnings.join('\n')}`);
                }
                if (parts.length > 0) {
                    this.showNotice(result.summary || 'Phase complete', parts.join('\n\n'));
                }

                await this.updateGameState(true);

            } catch (error) {
                this.showNotice('Cannot advance phase', error.message);
            }
        },

        /**
         * Execute NPC turn
         */
        async executeNPCTurn() {
            // No confirm() gate: clicking the button IS the request, and a
            // suppressed confirm() answers "no" forever, wedging the game.
            const who = this.gameState.currentPower;
            try {
                const result = await this.api.executeNPCTurn();
                // The turn's transcript, so the player can read exactly what
                // the computer did rather than diff the map by eye.
                const body = (result.transcript && result.transcript.length > 0)
                    ? result.transcript.join('\n')
                    : result.summary;
                this.showNotice(`${who}'s turn`, body);
                await this.updateGameState(true);
                await this.loadTerritories();
            } catch (error) {
                this.showNotice('NPC turn failed', error.message);
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
            // A heartbeat, not a firehose: one state GET every 2 seconds, and
            // updateGameState bails out before fetching anything else unless
            // the turn, power or phase actually moved. Action data refreshes
            // are driven by the player's own actions, which is the only way
            // the state changes mid-turn. A hidden tab skips the fetch
            // entirely -- nothing on screen can change, and the server holds
            // the session lock for every request it gets.
            this.pollingInterval = setInterval(() => {
                if (document.hidden) return;
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
        // selection follow automatically.

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
            // The flash stays on the territory path itself: it animates the
            // FILL, which neighbours cannot overpaint. The border glows for
            // eligible/destination territories live in the overlay layer
            // (see highlightShapes).
            if (this.ui.flash === shape.name) classes.push('flash-bad');

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
                // "Moved" is judged in screen pixels, not map units: a real
                // mouse drifts a pixel or two during an ordinary click, and a
                // map-unit threshold (~half a pixel when zoomed out) made the
                // swallow-guard below eat most territory selections. Until the
                // threshold is crossed nothing pans, so a click stays a click.
                const px = moveEvent.clientX - event.clientX;
                const py = moveEvent.clientY - event.clientY;
                if (!moved && Math.abs(px) <= 5 && Math.abs(py) <= 5) return;
                moved = true;
                this.view = this.clampView({
                    x: origin.x - px * scale, y: origin.y - py * scale,
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
        // Esc backs out of destination mode from anywhere.
        window.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.ui.mode === 'pickDest') {
                this.cancelTargeting();
            }
        });

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
