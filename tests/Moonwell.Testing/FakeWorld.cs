using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game a case runs against: it records what the runtime asks of it and moves time by hand.</summary>
    public sealed class FakeWorld : IWorld
    {
        /// <summary>Game seconds since the world was made.</summary>
        public float Time;

        /// <summary>The absolute time of the pending wake request; null when there is none.</summary>
        public float? WakeAt;

        /// <summary>
        /// How many seconds after the time asked for Advance delivers each wake; 0, the default, delivers it on time. The game wakes a
        /// timer at a frame, not at the exact time asked for: timers were seen running 0.0001 s off, and the time of slow frames is
        /// made up later in bursts.
        /// </summary>
        public float WakeLate;

        /// <summary>The slot of the player at this machine.</summary>
        public int LocalSlot;

        /// <summary>The slots the game gives the neutral players.</summary>
        public int NeutralAggressive = 24, NeutralVictim = 25, NeutralExtra = 26, NeutralPassive = 27;

        /// <summary>The lines printed on the screen.</summary>
        public List<string> Printed = new List<string>();

        /// <summary>The slot of each line printed to a player, -1 for a line printed to everyone.</summary>
        public List<int> PrintedSlots = new List<int>();

        /// <summary>The facing the last unit made by SpawnUnit was given.</summary>
        public float LastFacing;

        /// <summary>Each line printed to a player or to everyone.</summary>
        public List<string> PrintedTexts = new List<string>();

        /// <summary>The seconds of each line printed to a player or to everyone; below 0 for the game's own time.</summary>
        public List<float> PrintedSeconds = new List<float>();

        /// <summary>The lines appended to the log.</summary>
        public List<string> Logged = new List<string>();

        /// <summary>The damage the runtime asked the game to deal: the amount (whole), attack, ranged, and the game's attack, damage and weapon numbers.</summary>
        public List<string> Dealt = new List<string>();

        /// <summary>Whether the game deals no damage the runtime asks for, as for a target that cannot take it.</summary>
        public bool RefuseDamage;

        /// <summary>The events the runtime asked to hear, once each.</summary>
        public List<GameEvent> Listening = new List<GameEvent>();

        /// <summary>The destructables whose deaths the runtime asked to hear, once each.</summary>
        public List<destructable> Watched = new List<destructable>();

        /// <summary>The units the runtime removed.</summary>
        public List<unit> RemovedUnits = new List<unit>();

        /// <summary>The items the runtime removed.</summary>
        public List<item> RemovedItems = new List<item>();

        /// <summary>The destructables the runtime removed.</summary>
        public List<destructable> RemovedDestructables = new List<destructable>();

        /// <summary>Whether RemoveDestructable fires the death event of a watched destructable inside the call, as the game does (seen in a played map).</summary>
        public bool DeathOnRemove;

        /// <summary>The ability calls, as "add 42", "hide 42" or "remove 42", with the ability id in decimal.</summary>
        public List<string> AbilityCalls = new List<string>();

        /// <summary>The units UnitsInWorld returns.</summary>
        public List<unit> PlacedUnits = new List<unit>();

        /// <summary>The items ItemsInWorld returns.</summary>
        public List<item> PlacedItems = new List<item>();

        /// <summary>The destructables DestructablesInWorld returns.</summary>
        public List<destructable> PlacedDestructables = new List<destructable>();

        /// <summary>The type id of each unit the world made.</summary>
        private readonly Dictionary<unit, int> unitTypes = new Dictionary<unit, int>();

        /// <summary>The slot of the player that owns each unit the world made.</summary>
        private readonly Dictionary<unit, int> unitSlots = new Dictionary<unit, int>();

        /// <summary>The units whose owner reads null, as a unit the game removed a while ago does.</summary>
        private readonly HashSet<unit> ownerless = new HashSet<unit>();

        /// <summary>Whether each unit the world made is a hero.</summary>
        private readonly Dictionary<unit, bool> heroes = new Dictionary<unit, bool>();

        /// <summary>The units made illusions.</summary>
        private readonly HashSet<unit> illusions = new HashSet<unit>();

        /// <summary>The ability levels of each unit, keyed by ability id.</summary>
        private readonly Dictionary<unit, Dictionary<int, int>> abilityLevels = new Dictionary<unit, Dictionary<int, int>>();

        /// <summary>The type id of each item the world made.</summary>
        private readonly Dictionary<item, int> itemTypes = new Dictionary<item, int>();

        /// <summary>The type id of each destructable the world made.</summary>
        private readonly Dictionary<destructable, int> destructableTypes = new Dictionary<destructable, int>();

        /// <summary>Where PlaceDestructable put each destructable; one not placed stands at (100, -50).</summary>
        private readonly Dictionary<destructable, float[]> destructablePlaces = new Dictionary<destructable, float[]>();

        /// <summary>The calls that changed a unit's place or facing, as "move", "place", "turn" or "face-now".</summary>
        public List<string> UnitCalls = new List<string>();

        /// <summary>How many times a unit's life, mana, place or facing was read or set after the game had removed it, which the runtime must never do.</summary>
        public int GoneCalls;

        /// <summary>The facing each unit was last ordered to turn to; its own facing does not change by it, since turning takes time.</summary>
        public Dictionary<unit, float> TurnTargets = new Dictionary<unit, float>();

        /// <summary>How many times the runtime asked the game to kill a unit.</summary>
        public int KillCalls;

        /// <summary>The units that are paused.</summary>
        public List<unit> PausedUnits = new List<unit>();

        /// <summary>How many times a unit's scale was set.</summary>
        public int ScaleCalls;

        /// <summary>The red, green, blue and alpha each unit was last tinted with.</summary>
        public Dictionary<unit, int[]> Tints = new Dictionary<unit, int[]>();

        /// <summary>The drops of units the runtime removed (RemoveUnit): the game fires them after the call (seen 10-21), so they wait for the next Advance. The items are removed with the unit.</summary>
        private readonly List<KeyValuePair<unit, item>> pendingDrops = new List<KeyValuePair<unit, item>>();

        /// <summary>The units removed by the runtime in this frame, whose type id still reads until the next Advance.</summary>
        private readonly List<unit> pendingRemovals = new List<unit>();

        /// <summary>The units the game has removed but whose handles still answer: every number reads 0, as the game gives in the quarter second before the sweep.</summary>
        private readonly HashSet<unit> vanishedUnits = new HashSet<unit>();

        /// <summary>The items the game has removed but whose handles still answer: every number reads 0.</summary>
        private readonly HashSet<item> vanishedItems = new HashSet<item>();

        /// <summary>The units whose type flies (UNIT_TYPE_FLYING).</summary>
        private readonly HashSet<unit> flyers = new HashSet<unit>();

        /// <summary>The units that have been given the Amrf ability.</summary>
        private readonly HashSet<unit> crowFormed = new HashSet<unit>();

        /// <summary>The units that can show a fly height: those whose type flies and those that had Amrf added and removed.</summary>
        private readonly HashSet<unit> floaters = new HashSet<unit>();

        /// <summary>The units that have been killed.</summary>
        private readonly HashSet<unit> dead = new HashSet<unit>();

        /// <summary>The level of each unit, 1 until set.</summary>
        private readonly Dictionary<unit, int> levels = new Dictionary<unit, int>();

        /// <summary>The scale of each unit, 1 until set.</summary>
        private readonly Dictionary<unit, float> scales = new Dictionary<unit, float>();

        /// <summary>The fly height of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> flyHeights = new Dictionary<unit, float>();

        /// <summary>The orders given to units, as "immediate", "point" or "target", a space, and the order string.</summary>
        public List<string> Orders = new List<string>();

        /// <summary>The x and y of each point order, in turn.</summary>
        public List<float> OrderPoints = new List<float>();

        /// <summary>The widget each target order was given on, in turn.</summary>
        public List<widget> OrderTargets = new List<widget>();

        /// <summary>What the game answers to an order.</summary>
        public bool OrderAnswer = true;

        /// <summary>The inventory calls that changed something, as "give" or "drop".</summary>
        public List<string> InventoryCalls = new List<string>();

        /// <summary>How many times a slot of an inventory was read.</summary>
        public int ItemSlotReads;

        /// <summary>The calls that changed an item, as "charges 5", "owner 3 False" (slot and colour change) or "move".</summary>
        public List<string> ItemCalls = new List<string>();

        /// <summary>The charges of each item, 1 until set.</summary>
        private readonly Dictionary<item, int> itemCharges = new Dictionary<item, int>();

        /// <summary>The player slot of each item, 0 until set.</summary>
        private readonly Dictionary<item, int> itemSlots = new Dictionary<item, int>();

        /// <summary>The x and y of each item, 0 until placed.</summary>
        private readonly Dictionary<item, float[]> itemPlaces = new Dictionary<item, float[]>();

        /// <summary>The items in each unit's slots; null for an empty slot.</summary>
        private readonly Dictionary<unit, item[]> inventories = new Dictionary<unit, item[]>();

        /// <summary>The number of inventory slots of each unit, 6 until set.</summary>
        private readonly Dictionary<unit, int> inventorySizes = new Dictionary<unit, int>();

        /// <summary>The life of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> lives = new Dictionary<unit, float>();

        /// <summary>The most life of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> maxLives = new Dictionary<unit, float>();

        /// <summary>The mana of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> manas = new Dictionary<unit, float>();

        /// <summary>The most mana of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> maxManas = new Dictionary<unit, float>();

        /// <summary>The x of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> unitXs = new Dictionary<unit, float>();

        /// <summary>The y of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> unitYs = new Dictionary<unit, float>();

        /// <summary>The facing of each unit, 0 until set.</summary>
        private readonly Dictionary<unit, float> facings = new Dictionary<unit, float>();

        /// <summary>The player made for each slot, once it is asked for.</summary>
        private readonly Dictionary<int, player> players = new Dictionary<int, player>();

        /// <summary>The upgrade levels of each player slot, keyed by slot and then by upgrade id.</summary>
        private readonly Dictionary<int, Dictionary<int, int>> upgradeLevels = new Dictionary<int, Dictionary<int, int>>();

        /// <summary>The x of each player slot's start location, for the slots that have one set.</summary>
        private readonly Dictionary<int, float> startXs = new Dictionary<int, float>();

        /// <summary>The y of each player slot's start location, for the slots that have one set.</summary>
        private readonly Dictionary<int, float> startYs = new Dictionary<int, float>();

        /// <summary>The states set on players, by slot times 16 plus the state.</summary>
        private readonly Dictionary<int, int> states = new Dictionary<int, int>();

        /// <summary>The slot state of each slot SetSlot set.</summary>
        private readonly Dictionary<int, int> slotStates = new Dictionary<int, int>();

        /// <summary>Who controls each slot SetSlot set.</summary>
        private readonly Dictionary<int, int> controllers = new Dictionary<int, int>();

        /// <summary>The slot of each player made.</summary>
        private readonly Dictionary<player, int> slots = new Dictionary<player, int>();

        /// <summary>The names set for player slots.</summary>
        private readonly Dictionary<int, string> names = new Dictionary<int, string>();

        /// <summary>The colours set for player slots.</summary>
        private readonly Dictionary<int, int> colors = new Dictionary<int, int>();

        /// <summary>The teams set for player slots.</summary>
        private readonly Dictionary<int, int> teams = new Dictionary<int, int>();

        /// <summary>The races set for player slots.</summary>
        private readonly Dictionary<int, int> races = new Dictionary<int, int>();

        /// <summary>The alliances set, keyed by the source slot times 32 plus the other slot.</summary>
        private readonly Dictionary<int, int> alliances = new Dictionary<int, int>();

        /// <summary>The tech limits set, keyed by slot, then by type id.</summary>
        private readonly Dictionary<int, Dictionary<int, int>> techMaxes = new Dictionary<int, Dictionary<int, int>>();

        /// <summary>The declared values of each type, keyed by type id, then by field id, as the build's table gives them.</summary>
        private readonly Dictionary<int, Dictionary<string, object>> typeValues = new Dictionary<int, Dictionary<string, object>>();

        /// <summary>The name of each unit, "" until set.</summary>
        private readonly Dictionary<unit, string> unitNames = new Dictionary<unit, string>();

        /// <summary>The proper name of each hero, "" until set.</summary>
        private readonly Dictionary<unit, string> properNames = new Dictionary<unit, string>();

        /// <summary>Whether a call on a unit or an item throws, for a case that checks the runtime makes none.</summary>
        public bool CallsThrow;

        /// <summary>Makes a fresh world for one case and makes it the runtime's world; as the runtime does at its start, it listens for pick-ups and drops.</summary>
        public static FakeWorld Install()
        {
            var world = new FakeWorld();
            Runtime.Reset(world);
            Interactions.ResetSettings();
            Runtime.ListenForCarriers();
            return world;
        }

        /// <summary>Makes a unit with a type id, a slot and a hero flag, not placed in the world.</summary>
        public unit NewUnit(int typeId, int slot = 0, bool hero = false)
        {
            var created = new unit();
            unitTypes[created] = typeId;
            unitSlots[created] = slot;
            heroes[created] = hero;
            return created;
        }

        /// <summary>Makes an item with a type id, not placed in the world.</summary>
        public item NewItem(int typeId)
        {
            var created = new item();
            itemTypes[created] = typeId;
            return created;
        }

        /// <summary>Makes a destructable with a type id, not placed in the world.</summary>
        public destructable NewDestructable(int typeId)
        {
            var created = new destructable();
            destructableTypes[created] = typeId;
            return created;
        }

        /// <summary>Puts a destructable at a place; one not placed stands at (100, -50).</summary>
        public void PlaceDestructable(destructable d, float x, float y)
        {
            destructablePlaces[d] = new float[] { x, y };
        }

        /// <summary>Makes a unit vanish the way the game does: its type id and its owner read 0 and null, and nothing is told.</summary>
        public void VanishUnit(unit u)
        {
            unitTypes[u] = 0;
            ownerless.Add(u);
            vanishedUnits.Add(u);
        }

        /// <summary>Makes an item vanish the way the game does: its type id reads 0 and nothing is told.</summary>
        public void VanishItem(item i)
        {
            itemTypes[i] = 0;
            vanishedItems.Add(i);
        }

        /// <summary>Makes a destructable vanish the way the game does: its type id reads 0 and nothing is told.</summary>
        public void VanishDestructable(destructable d)
        {
            destructableTypes[d] = 0;
        }

        /// <summary>Makes a unit one whose type flies: CanFly answers true and it shows a fly height at once.</summary>
        public void MakeFlying(unit u)
        {
            flyers.Add(u);
            floaters.Add(u);
        }

        /// <summary>Whether the unit would be seen above the ground: its type flies, or Amrf was added and removed (seen 10-21: a ground unit's fly height reads back but shows only then), and a height is set.</summary>
        public bool Floats(unit u)
        {
            return (floaters.Contains(u) || AbilityLevel(u, CrowForm) > 0) && flyHeights.TryGetValue(u, out var height) && height != 0.0f;
        }

        /// <summary>Sets the level an ability has on a unit.</summary>
        public void SetAbilityLevel(unit u, int abilityId, int level)
        {
            if (!abilityLevels.TryGetValue(u, out var levels))
            {
                levels = new Dictionary<int, int>();
                abilityLevels[u] = levels;
            }

            levels[abilityId] = level;
        }

        /// <summary>Sets the level a player slot has of an upgrade.</summary>
        public void SetUpgradeLevel(int slot, int upgradeId, int level)
        {
            if (!upgradeLevels.TryGetValue(slot, out var levels))
            {
                levels = new Dictionary<int, int>();
                upgradeLevels[slot] = levels;
            }

            levels[upgradeId] = level;
        }

        /// <summary>Sets the race of a player slot.</summary>
        public void SetRace(int slot, Race race)
        {
            PlayerAt(slot);
            races[slot] = (int)race;
        }

        /// <summary>Sets the start location of a player slot.</summary>
        public void SetStart(int slot, float x, float y)
        {
            startXs[slot] = x;
            startYs[slot] = y;
        }

        /// <summary>Sets a declared value of a type, as the build's table holds it.</summary>
        public void SetTypeValue(int typeId, string field, object value)
        {
            if (!typeValues.TryGetValue(typeId, out var values))
            {
                values = new Dictionary<string, object>();
                typeValues[typeId] = values;
            }

            values[field] = value;
        }

        /// <summary>A declared value of a type; null when none was set.</summary>
        public object? TypeValue(int typeId, string field)
        {
            if (typeValues.TryGetValue(typeId, out var values) && values.TryGetValue(field, out var value))
            {
                return value;
            }

            return null;
        }

        /// <summary>Moves time forward by the seconds, calling Runtime.Wake at each wake request on the way, WakeLate after the time it asked for. The frame of the last calls ends first: the drops that waited for it fire, and the units removed in it read type id 0.</summary>
        public void Advance(float seconds)
        {
            var target = Time + seconds;
            DeliverPendingDrops();
            EndPendingRemovals();
            while (WakeAt.HasValue && WakeAt.Value + WakeLate <= target + 0.001f)
            {
                Time = WakeAt.Value + WakeLate;
                WakeAt = null;
                Runtime.Wake();
            }

            Time = target;
        }

        /// <summary>Makes the units removed in the frame that ends read type id 0.</summary>
        private void EndPendingRemovals()
        {
            foreach (var removed in pendingRemovals)
            {
                unitTypes[removed] = 0;
            }

            pendingRemovals.Clear();
        }

        /// <summary>Fires the drop events that waited for the end of the call that removed their unit, when they are listened for, and then removes the items with the unit.</summary>
        private void DeliverPendingDrops()
        {
            if (pendingDrops.Count == 0)
            {
                return;
            }

            var waiting = new List<KeyValuePair<unit, item>>(pendingDrops);
            pendingDrops.Clear();
            foreach (var drop in waiting)
            {
                if (Listening.Contains(GameEvent.Drop))
                {
                    Inbound.Dropped(drop.Key, drop.Value);
                }

                VanishItem(drop.Value);
            }
        }

        /// <summary>The game seconds since the world was made; in a development build, Moonwell's time on its clock (Runtime.Clock), as NativeWorld's.</summary>
        public float Now()
        {
#if DEBUG
            return Runtime.Clock.Now(Time);
#else
            return Time;
#endif
        }

        /// <summary>Requests a wake the seconds from now; replaces any earlier request. In a development build the seconds are Moonwell's, and a request while its clock is halted drops the wake until the clock re-arms it, as NativeWorld's.</summary>
        public void WakeIn(float seconds)
        {
#if DEBUG
            if (Runtime.Clock.Arm(seconds, out var gameSeconds))
            {
                WakeAt = Time + gameSeconds;
            }
            else
            {
                WakeAt = null;
            }
#else
            WakeAt = Time + seconds;
#endif
        }

        /// <summary>Records that the runtime asked to hear an event, once.</summary>
        public void Listen(GameEvent gameEvent)
        {
            if (!Listening.Contains(gameEvent))
            {
                Listening.Add(gameEvent);
            }
        }

        /// <summary>Records the damage; unless refused, delivers it to Inbound as the game does, inside the call, when the runtime listens for damage. The game's event says no attack even for one dealt as an attack.</summary>
        public bool DealDamage(unit source, unit target, float amount, bool attack, bool ranged, int attackType, int damageType, int weaponType)
        {
            Dealt.Add((int)amount + " attack:" + attack + " ranged:" + ranged + " " + attackType + " " + damageType + " " + weaponType);
            if (RefuseDamage)
            {
                return false;
            }

            if (Listening.Contains(GameEvent.Damage))
            {
                Inbound.Damaged(target, source, amount, false);
            }

            return true;
        }

        /// <summary>Records that the runtime asked to hear a destructable's death, once.</summary>
        public void WatchDeath(destructable destructable)
        {
            if (!Watched.Contains(destructable))
            {
                Watched.Add(destructable);
            }
        }

        /// <summary>Records a line printed on the screen.</summary>
        public void Print(string text)
        {
            Printed.Add(text);
        }

        /// <summary>The slot state of a player; Empty until SetSlot sets it.</summary>
        public int SlotStateOf(player player)
        {
            return slotStates.TryGetValue(SlotOf(player), out var state) ? state : 0;
        }

        /// <summary>Who controls a player; User until SetSlot sets it.</summary>
        public int ControllerOf(player player)
        {
            return controllers.TryGetValue(SlotOf(player), out var control) ? control : 0;
        }

        /// <summary>Sets a slot's state and who controls it.</summary>
        public void SetSlot(int slot, SlotState state, Controller controller)
        {
            PlayerAt(slot);
            slotStates[slot] = (int)state;
            controllers[slot] = (int)controller;
        }

        /// <summary>A state of a player; 0 until it is set.</summary>
        public int StateOf(player player, PlayerState state)
        {
            return states.TryGetValue(SlotOf(player) * 16 + (int)state, out var value) ? value : 0;
        }

        /// <summary>Sets a state of a player.</summary>
        public void SetState(player player, PlayerState state, int value)
        {
            states[SlotOf(player) * 16 + (int)state] = value;
        }

        /// <summary>Records a line printed to one player.</summary>
        public void PrintTo(player player, string text, float seconds)
        {
            PrintedSlots.Add(SlotOf(player));
            PrintedTexts.Add(text);
            PrintedSeconds.Add(seconds);
        }

        /// <summary>Records a line printed to everyone.</summary>
        public void PrintToAll(string text, float seconds)
        {
            PrintedSlots.Add(-1);
            PrintedTexts.Add(text);
            PrintedSeconds.Add(seconds);
        }

        /// <summary>Records a line appended to the log.</summary>
        public void Log(string text)
        {
            Logged.Add(text);
        }

        /// <summary>The player in a slot; one is made the first time its slot is asked for.</summary>
        public player PlayerAt(int slot)
        {
            if (players.TryGetValue(slot, out var found))
            {
                return found;
            }

            found = new player();
            players[slot] = found;
            slots[found] = slot;
            return found;
        }

        /// <summary>The slot of a player the world made, or -1 for a player it did not make.</summary>
        public int SlotOf(player player)
        {
            if (slots.TryGetValue(player, out var slot))
            {
                return slot;
            }

            return -1;
        }

        /// <summary>The slot of the neutral hostile player, from NeutralAggressive.</summary>
        public int NeutralAggressiveSlot()
        {
            return NeutralAggressive;
        }

        /// <summary>The slot of the neutral victim player, from NeutralVictim.</summary>
        public int NeutralVictimSlot()
        {
            return NeutralVictim;
        }

        /// <summary>The slot of the neutral extra player, from NeutralExtra.</summary>
        public int NeutralExtraSlot()
        {
            return NeutralExtra;
        }

        /// <summary>The slot of the neutral passive player, from NeutralPassive.</summary>
        public int NeutralPassiveSlot()
        {
            return NeutralPassive;
        }

        /// <summary>The x of a player's start location; 0 when none is set.</summary>
        public float StartXOf(player player)
        {
            return startXs.TryGetValue(SlotOf(player), out var x) ? x : 0f;
        }

        /// <summary>The y of a player's start location; 0 when none is set.</summary>
        public float StartYOf(player player)
        {
            return startYs.TryGetValue(SlotOf(player), out var y) ? y : 0f;
        }

        /// <summary>Whether a player is the one at this machine: the player in LocalSlot.</summary>
        public bool IsLocal(player player)
        {
            return SlotOf(player) == LocalSlot;
        }

        /// <summary>A copy of the units placed in the world.</summary>
        public List<unit> UnitsInWorld()
        {
            return new List<unit>(PlacedUnits);
        }

        /// <summary>A copy of the items placed in the world.</summary>
        public List<item> ItemsInWorld()
        {
            return new List<item>(PlacedItems);
        }

        /// <summary>A copy of the destructables placed in the world.</summary>
        public List<destructable> DestructablesInWorld()
        {
            return new List<destructable>(PlacedDestructables);
        }

        /// <summary>Makes a unit owned by the owner's slot at the place, and keeps the facing in LastFacing.</summary>
        public unit SpawnUnit(player owner, int typeId, float x, float y, float facing)
        {
            LastFacing = facing;
            var created = NewUnit(typeId, SlotOf(owner));
            unitXs[created] = x;
            unitYs[created] = y;
            return created;
        }

        /// <summary>Makes an item; the place is not kept.</summary>
        public item SpawnItem(int typeId, float x, float y)
        {
            var created = NewItem(typeId);
            itemPlaces[created] = new float[] { x, y };
            return created;
        }

        /// <summary>The type id of a unit; 0 once it is removed or vanished.</summary>
        public int UnitTypeId(unit unit)
        {
            Refuse();
            if (unitTypes.TryGetValue(unit, out var typeId))
            {
                return typeId;
            }

            return 0;
        }

        /// <summary>The type id of an item; 0 once it is removed or vanished.</summary>
        public int ItemTypeId(item item)
        {
            Refuse();
            if (itemTypes.TryGetValue(item, out var typeId))
            {
                return typeId;
            }

            return 0;
        }

        /// <summary>The type id of a destructable; 0 once it is removed or vanished.</summary>
        public int DestructableTypeId(destructable destructable)
        {
            if (destructableTypes.TryGetValue(destructable, out var typeId))
            {
                return typeId;
            }

            return 0;
        }

        /// <summary>Records a unit removed by the runtime. Its type id still reads in this frame, as the game's does after RemoveUnit, and reads 0 from the next Advance, after the drop events (seen 10-21: 0 by 0.05 s later; seen 10-26: the drops fire after the call while it still reads). The unit's items leave its slots inside the call, at its place; their drop events come after the call, at the next Advance, and the items are removed with the unit (seen 10-21 for a hero).</summary>
        public void DeleteUnit(unit unit)
        {
            RemovedUnits.Add(unit);
            if (inventories.TryGetValue(unit, out var slots))
            {
                for (var i = 0; i < slots.Length; i++)
                {
                    var carried = slots[i];
                    if (carried != null)
                    {
                        slots[i] = null!;
                        LandAt(unit, carried);
                        pendingDrops.Add(new KeyValuePair<unit, item>(unit, carried));
                    }
                }
            }

            pendingRemovals.Add(unit);
        }

        /// <summary>Puts an item that leaves a unit's slots on the ground at the unit's place.</summary>
        private void LandAt(unit unit, item item)
        {
            unitXs.TryGetValue(unit, out var x);
            unitYs.TryGetValue(unit, out var y);
            itemPlaces[item] = new float[] { x, y };
        }

        /// <summary>Records an item removed by the runtime, and reads its type id as 0.</summary>
        public void DeleteItem(item item)
        {
            RemovedItems.Add(item);
            itemTypes[item] = 0;
        }

        /// <summary>The x of a destructable; 100 unless placed, and 0 once it is removed or vanished.</summary>
        public float DestructableX(destructable destructable)
        {
            if (DestructableTypeId(destructable) == 0)
            {
                return 0.0f;
            }

            return destructablePlaces.TryGetValue(destructable, out var place) ? place[0] : 100f;
        }

        /// <summary>The y of a destructable; -50 unless placed, and 0 once it is removed or vanished.</summary>
        public float DestructableY(destructable destructable)
        {
            if (DestructableTypeId(destructable) == 0)
            {
                return 0.0f;
            }

            return destructablePlaces.TryGetValue(destructable, out var place) ? place[1] : -50f;
        }

        /// <summary>Records a destructable removed by the runtime, and reads its type id as 0.</summary>
        public void DeleteDestructable(destructable destructable)
        {
            RemovedDestructables.Add(destructable);
            if (DeathOnRemove && Watched.Contains(destructable))
            {
                Inbound.DestructableDied(destructable);
            }

            destructableTypes[destructable] = 0;
        }

        /// <summary>Whether a unit was made as a hero.</summary>
        public bool IsHero(unit unit)
        {
            Refuse();
            return heroes.TryGetValue(unit, out var hero) && hero;
        }

        /// <summary>Whether a unit was made an illusion with MakeIllusion.</summary>
        public bool IsIllusion(unit unit)
        {
            Refuse();
            return illusions.Contains(unit);
        }

        /// <summary>Makes a unit an illusion, as Mirror Image makes one of its hero, with the hero's type id (seen 10-26).</summary>
        public void MakeIllusion(unit u)
        {
            illusions.Add(u);
        }

        /// <summary>The player owning a unit; null for a vanished unit, slot 0 for a unit the world did not make.</summary>
        public player OwnerOf(unit unit)
        {
            Refuse();
            if (ownerless.Contains(unit))
            {
                return null!;
            }

            if (unitSlots.TryGetValue(unit, out var slot))
            {
                return PlayerAt(slot);
            }

            return PlayerAt(0);
        }

        /// <summary>The level set on a unit for an ability; 0 when none was set.</summary>
        public int AbilityLevel(unit unit, int abilityId)
        {
            if (abilityLevels.TryGetValue(unit, out var levels) && levels.TryGetValue(abilityId, out var level))
            {
                return level;
            }

            return 0;
        }

        /// <summary>The level set for a player's slot for an upgrade; 0 when none was set.</summary>
        public int UpgradeLevel(player p, int upgradeId)
        {
            if (upgradeLevels.TryGetValue(SlotOf(p), out var levels) && levels.TryGetValue(upgradeId, out var level))
            {
                return level;
            }

            return 0;
        }

        /// <summary>Sets the level a player's slot has of an upgrade.</summary>
        public void Research(player player, int upgradeId, int level)
        {
            SetUpgradeLevel(SlotOf(player), upgradeId, level);
        }

        /// <summary>The tech limit set for a player's slot; int.MaxValue, the game's "no limit" for a unit type, when none was set.</summary>
        public int TechMaxOf(player player, int typeId)
        {
            if (techMaxes.TryGetValue(SlotOf(player), out var maxes) && maxes.TryGetValue(typeId, out var max))
            {
                return max;
            }

            return int.MaxValue;
        }

        /// <summary>Sets the tech limit of a player's slot.</summary>
        public void SetTechMax(player player, int typeId, int max)
        {
            var slot = SlotOf(player);
            if (!techMaxes.TryGetValue(slot, out var maxes))
            {
                maxes = new Dictionary<int, int>();
                techMaxes[slot] = maxes;
            }

            maxes[typeId] = max;
        }

        /// <summary>Records a call that makes an ability available or not, as "available slot id on|off".</summary>
        public void MakeAvailable(player player, int abilityId, bool available)
        {
            AbilityCalls.Add("available " + SlotOf(player) + " " + abilityId + (available ? " on" : " off"));
        }

        /// <summary>The name set for a player's slot; "Player n", counted from 1, until one is set.</summary>
        public string NameOf(player player)
        {
            var slot = SlotOf(player);
            return names.TryGetValue(slot, out var name) ? name : "Player " + (slot + 1);
        }

        /// <summary>Sets the name of a player's slot.</summary>
        public void Rename(player player, string name)
        {
            names[SlotOf(player)] = name;
        }

        /// <summary>The colour set for a player's slot; the slot's own number until one is set.</summary>
        public int ColorOf(player player)
        {
            var slot = SlotOf(player);
            return colors.TryGetValue(slot, out var color) ? color : slot;
        }

        /// <summary>Sets the colour of a player's slot.</summary>
        public void Recolor(player player, int color)
        {
            colors[SlotOf(player)] = color;
        }

        /// <summary>The team set for a player's slot; 0 until one is set.</summary>
        public int TeamOf(player player)
        {
            return teams.TryGetValue(SlotOf(player), out var team) ? team : 0;
        }

        /// <summary>Sets the team of a player's slot.</summary>
        public void MoveToTeam(player player, int team)
        {
            teams[SlotOf(player)] = team;
        }

        /// <summary>The race set for a player's slot; Human until SetRace sets it.</summary>
        public int RaceOf(player player)
        {
            return races.TryGetValue(SlotOf(player), out var race) ? race : 0;
        }

        /// <summary>The alliance set from one slot to another; None until one is set.</summary>
        public int AllianceOf(player source, player other)
        {
            return alliances.TryGetValue(SlotOf(source) * 32 + SlotOf(other), out var alliance) ? alliance : 0;
        }

        /// <summary>Sets the alliance from one slot to another.</summary>
        public void SetAllianceOf(player source, player other, int alliance)
        {
            alliances[SlotOf(source) * 32 + SlotOf(other)] = alliance;
        }

        /// <summary>Whether the other is an ally: the same slot, or one the source is passive to.</summary>
        public bool IsAllyOf(player source, player other)
        {
            return SlotOf(source) == SlotOf(other) || (AllianceOf(source, other) & (int)Alliance.Passive) != 0;
        }

        /// <summary>Whether the other is an enemy: not an ally.</summary>
        public bool IsEnemyOf(player source, player other)
        {
            return !IsAllyOf(source, other);
        }

        /// <summary>Records an add-ability call.</summary>
        public void AddAbility(unit unit, int abilityId)
        {
            AbilityCalls.Add("add " + abilityId);
            if (abilityId == CrowForm)
            {
                crowFormed.Add(unit);
            }
        }

        /// <summary>Records a hide-ability call.</summary>
        public void HideAbility(unit unit, int abilityId)
        {
            AbilityCalls.Add("hide " + abilityId);
        }

        /// <summary>Records a remove-ability call.</summary>
        public void RemoveAbility(unit unit, int abilityId)
        {
            AbilityCalls.Add("remove " + abilityId);
            if (abilityId == CrowForm && crowFormed.Remove(unit))
            {
                floaters.Add(unit);
            }
        }

        /// <summary>The id of Crow Form (Amrf), whose adding and removing lets a ground unit show a fly height.</summary>
        public static readonly int CrowForm = ObjectTypes.FourCC("Amrf");

        /// <summary>Throws when CallsThrow is set: the runtime called the game about a unit or an item.</summary>
        private void Refuse()
        {
            if (CallsThrow)
            {
                throw new ExpectationFailed("the runtime called the game about a unit or an item");
            }
        }

        /// <summary>Counts a call on an item the world has removed.</summary>
        private void NoteGoneItem(item item)
        {
            if (ItemTypeId(item) == 0)
            {
                GoneCalls++;
            }
        }

        /// <summary>The charges of an item; 1 until set.</summary>
        public int ChargesOf(item item)
        {
            NoteGoneItem(item);
            if (vanishedItems.Contains(item))
            {
                return 0;
            }

            return itemCharges.TryGetValue(item, out var charges) ? charges : 1;
        }

        /// <summary>Sets the charges of an item and records "charges".</summary>
        public void SetCharges(item item, int charges)
        {
            NoteGoneItem(item);
            itemCharges[item] = charges;
            ItemCalls.Add("charges " + charges);
        }

        /// <summary>The player that owns an item; slot 0 until set, null once LoseItemOwner has been called for it.</summary>
        public player ItemOwnerOf(item item)
        {
            NoteGoneItem(item);
            var slot = itemSlots.TryGetValue(item, out var kept) ? kept : 0;
            if (slot < 0)
            {
                return null!;
            }

            return PlayerAt(slot);
        }

        /// <summary>Makes the game give no owner for an item, as it may for an item nobody owns.</summary>
        public void LoseItemOwner(item item)
        {
            itemSlots[item] = -1;
        }

        /// <summary>Whether a unit has an item in a slot.</summary>
        public bool CarriesItem(unit unit, item item)
        {
            NoteGone(unit);
            var slots = SlotsOf(unit);
            for (var i = 0; i < slots.Length; i++)
            {
                if (slots[i] == item)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Moves an item to another, empty slot of the unit, as the game does when the player drags it; the game fires a drop for it (seen 10-15) while the unit still has the item.</summary>
        public void MoveItemSlot(unit unit, item item, int toSlot)
        {
            var slots = SlotsOf(unit);
            for (var i = 0; i < slots.Length; i++)
            {
                if (slots[i] == item)
                {
                    slots[i] = null!;
                    slots[toSlot] = item;
                    if (Listening.Contains(GameEvent.Drop))
                    {
                        Inbound.Dropped(unit, item);
                    }

                    return;
                }
            }
        }

        /// <summary>Sets the player of an item and records "owner" with the slot and False, since the item's colour is left alone.</summary>
        public void SetItemOwner(item item, player player)
        {
            NoteGoneItem(item);
            itemSlots[item] = SlotOf(player);
            ItemCalls.Add("owner " + SlotOf(player) + " False");
        }

        /// <summary>The x of an item; 0 until placed.</summary>
        public float ItemX(item item)
        {
            NoteGoneItem(item);
            return !vanishedItems.Contains(item) && itemPlaces.TryGetValue(item, out var place) ? place[0] : 0.0f;
        }

        /// <summary>The y of an item; 0 until placed.</summary>
        public float ItemY(item item)
        {
            NoteGoneItem(item);
            return !vanishedItems.Contains(item) && itemPlaces.TryGetValue(item, out var place) ? place[1] : 0.0f;
        }

        /// <summary>The place (x, y) the game puts every moved item at when set, as it snaps a place on a tree or past the edge to a pathable spot (seen 10-21); null puts it where it was asked.</summary>
        public float[]? ItemLandsAt;

        /// <summary>Moves an item to the place given, or to <see cref="ItemLandsAt"/> when set, and records "move".</summary>
        public void MoveItem(item item, float x, float y)
        {
            NoteGoneItem(item);
            itemPlaces[item] = ItemLandsAt != null ? new float[] { ItemLandsAt[0], ItemLandsAt[1] } : new float[] { x, y };
            ItemCalls.Add("move");
        }

        /// <summary>Puts an item in a slot of a unit without the game telling anyone, as when the item was there before the runtime came.</summary>
        public void PutInSlot(unit unit, int slot, item item)
        {
            SlotsOf(unit)[slot] = item;
        }

        /// <summary>Counts a call on a unit the world has removed.</summary>
        private void NoteGone(unit unit)
        {
            if (UnitTypeId(unit) == 0)
            {
                GoneCalls++;
            }
        }

        /// <summary>A value kept for a unit; 0 when none was set.</summary>
        private float Kept(Dictionary<unit, float> values, unit unit)
        {
            NoteGone(unit);
            if (vanishedUnits.Contains(unit))
            {
                return 0.0f;
            }

            return values.TryGetValue(unit, out var value) ? value : 0.0f;
        }

        /// <summary>The life of a unit; 0 until set.</summary>
        public float LifeOf(unit unit)
        {
            return Kept(lives, unit);
        }

        /// <summary>Sets the life of a unit; above the most life, once one is set, it is clamped to it (seen 10-21); 0.405 or less kills it as KillUnit does (seen 10-20).</summary>
        public void SetLife(unit unit, float life)
        {
            NoteGone(unit);
            if (life <= 0.405f)
            {
                Die(unit);
                return;
            }

            lives[unit] = maxLives.TryGetValue(unit, out var most) && life > most ? most : life;
        }

        /// <summary>The most life of a unit; 0 until set.</summary>
        public float MaxLifeOf(unit unit)
        {
            return Kept(maxLives, unit);
        }

        /// <summary>Sets the most life of a unit, as given.</summary>
        public void SetMaxLife(unit unit, float life)
        {
            NoteGone(unit);
            maxLives[unit] = (int)life;
            if (lives.TryGetValue(unit, out var kept) && kept > (int)life)
            {
                lives[unit] = (int)life;
            }
        }

        /// <summary>The mana of a unit; 0 until set.</summary>
        public float ManaOf(unit unit)
        {
            return Kept(manas, unit);
        }

        /// <summary>Sets the mana of a unit; above the most mana, once one is set, it is clamped to it (seen 10-21).</summary>
        public void SetMana(unit unit, float mana)
        {
            NoteGone(unit);
            manas[unit] = maxManas.TryGetValue(unit, out var most) && mana > most ? most : mana;
        }

        /// <summary>The most mana of a unit; 0 until set.</summary>
        public float MaxManaOf(unit unit)
        {
            return Kept(maxManas, unit);
        }

        /// <summary>Sets the most mana of a unit, as given.</summary>
        public void SetMaxMana(unit unit, float mana)
        {
            NoteGone(unit);
            maxManas[unit] = (int)mana;
            if (manas.TryGetValue(unit, out var kept) && kept > (int)mana)
            {
                manas[unit] = (int)mana;
            }
        }

        /// <summary>The x of a unit; 0 until set.</summary>
        public float UnitX(unit unit)
        {
            return Kept(unitXs, unit);
        }

        /// <summary>The y of a unit; 0 until set.</summary>
        public float UnitY(unit unit)
        {
            return Kept(unitYs, unit);
        }

        /// <summary>Sets the player of a unit and records "owner" with the slot and True, since the unit's colour changes too.</summary>
        public void SetOwner(unit unit, player player)
        {
            NoteGone(unit);
            unitSlots[unit] = SlotOf(player);
            UnitCalls.Add("owner " + SlotOf(player) + " True");
        }

        /// <summary>Puts a unit at the place, and records a "move".</summary>
        public void MoveUnit(unit unit, float x, float y)
        {
            NoteGone(unit);
            UnitCalls.Add("move");
            unitXs[unit] = x;
            unitYs[unit] = y;
        }

        /// <summary>Puts a unit at the place, and records a "place".</summary>
        public void PlaceUnit(unit unit, float x, float y)
        {
            NoteGone(unit);
            UnitCalls.Add("place");
            unitXs[unit] = x;
            unitYs[unit] = y;
        }

        /// <summary>The facing of a unit; 0 until set.</summary>
        public float FacingOf(unit unit)
        {
            return Kept(facings, unit);
        }

        /// <summary>Records a "turn" and the facing asked for in TurnTargets; the unit's facing stays, since a turn takes time.</summary>
        public void TurnUnit(unit unit, float degrees)
        {
            NoteGone(unit);
            UnitCalls.Add("turn");
            TurnTargets[unit] = degrees;
        }

        /// <summary>Sets the facing of a unit, and records a "face-now".</summary>
        public void FaceUnitNow(unit unit, float degrees)
        {
            NoteGone(unit);
            UnitCalls.Add("face-now");
            facings[unit] = degrees;
        }

        /// <summary>Sets the level the world gives a unit.</summary>
        public void SetLevel(unit unit, int level)
        {
            levels[unit] = level;
        }

        /// <summary>The level of a unit; 1 until set.</summary>
        public int LevelOf(unit unit)
        {
            NoteGone(unit);
            if (vanishedUnits.Contains(unit))
            {
                return 0;
            }

            return levels.TryGetValue(unit, out var level) ? level : 1;
        }

        /// <summary>Whether a unit has not been killed.</summary>
        public bool IsAlive(unit unit)
        {
            NoteGone(unit);
            return !dead.Contains(unit);
        }

        /// <summary>Kills a unit as the game does: the unit is dead with life 0, and when deaths are listened for, its death is delivered inside the call, with no killer; killing a dead unit does nothing.</summary>
        public void SlayUnit(unit unit)
        {
            NoteGone(unit);
            KillCalls++;
            Die(unit);
        }

        /// <summary>The death itself, for KillUnit and for a life set to 0.405 or less: life 0, the death inside the call when deaths are listened for, and nothing for a unit already dead (seen 10-21).</summary>
        private void Die(unit unit)
        {
            if (!dead.Add(unit))
            {
                // A second KillUnit on the dead unit fires nothing (seen 10-21).
                return;
            }

            lives[unit] = 0.0f;
            if (Listening.Contains(GameEvent.Death))
            {
                Inbound.Died(unit, null);
            }

            DropAll(unit);
        }

        /// <summary>Empties a dead unit's slots as the game drops the items of a unit that is not a hero, at its place, with no drop event (seen 10-21); a hero keeps its items.</summary>
        private void DropAll(unit unit)
        {
            if (heroes.TryGetValue(unit, out var isHero) && isHero)
            {
                return;
            }

            var slots = SlotsOf(unit);
            for (var i = 0; i < slots.Length; i++)
            {
                var carried = slots[i];
                if (carried != null)
                {
                    slots[i] = null!;
                    LandAt(unit, carried);
                }
            }
        }

        /// <summary>The calls that changed a hero, in turn, as a word and its values: "experience 500 True", "level 5 True", "strength 30 True", "skill-points 3", "revive 100 200 True".</summary>
        public List<string> HeroCalls = new List<string>();

        /// <summary>The experience of each hero, 0 until set.</summary>
        private readonly Dictionary<unit, int> experiences = new Dictionary<unit, int>();

        /// <summary>The base strength, agility and intelligence of each hero, 0 until set.</summary>
        private readonly Dictionary<unit, int[]> stats = new Dictionary<unit, int[]>();

        /// <summary>The bonus to strength, agility and intelligence of each hero, 0 until set.</summary>
        private readonly Dictionary<unit, int[]> statBonuses = new Dictionary<unit, int[]>();

        /// <summary>The skill points of each hero, 0 until set.</summary>
        private readonly Dictionary<unit, int> skillPoints = new Dictionary<unit, int>();

        /// <summary>Sets the base strength, agility and intelligence the world gives a hero.</summary>
        public void SetStats(unit hero, int strength, int agility, int intelligence)
        {
            stats[hero] = new int[] { strength, agility, intelligence };
        }

        /// <summary>Sets the bonus to strength, agility and intelligence the world gives a hero.</summary>
        public void SetStatBonuses(unit hero, int strength, int agility, int intelligence)
        {
            statBonuses[hero] = new int[] { strength, agility, intelligence };
        }

        /// <summary>Sets the skill points the world gives a hero.</summary>
        public void SetSkillPoints(unit hero, int points)
        {
            skillPoints[hero] = points;
        }

        /// <summary>The experience of a hero; 0 until set.</summary>
        public int ExperienceOf(unit hero)
        {
            NoteGone(hero);
            return !vanishedUnits.Contains(hero) && experiences.TryGetValue(hero, out var experience) ? experience : 0;
        }

        /// <summary>Sets the experience of a hero, and records an "experience".</summary>
        public void SetExperience(unit hero, int experience, bool showEffect)
        {
            NoteGone(hero);
            HeroCalls.Add("experience " + experience + " " + Flag(showEffect));
            experiences[hero] = experience;
        }

        /// <summary>Gives a hero the level, and records a "level".</summary>
        public void LevelHero(unit hero, int level, bool showEffect)
        {
            NoteGone(hero);
            HeroCalls.Add("level " + level + " " + Flag(showEffect));
            levels[hero] = level;
        }

        /// <summary>A stat of a hero, 0 until set: which is 0 for strength, 1 for agility and 2 for intelligence.</summary>
        private int StatOf(unit hero, int which, bool withBonus)
        {
            NoteGone(hero);
            if (vanishedUnits.Contains(hero))
            {
                return 0;
            }

            var value = stats.TryGetValue(hero, out var set) ? set[which] : 0;
            if (withBonus && statBonuses.TryGetValue(hero, out var bonus))
            {
                value += bonus[which];
            }

            return value;
        }

        /// <summary>Sets a base stat of a hero, and records its name with the value and that it is for good.</summary>
        private void SetStat(unit hero, int which, string name, int value)
        {
            NoteGone(hero);
            HeroCalls.Add(name + " " + value + " " + Flag(true));
            if (!stats.TryGetValue(hero, out var set))
            {
                set = new int[3];
                stats[hero] = set;
            }

            set[which] = value;
        }

        /// <summary>The strength of a hero.</summary>
        public int StrengthOf(unit hero, bool withBonus)
        {
            return StatOf(hero, 0, withBonus);
        }

        /// <summary>The agility of a hero.</summary>
        public int AgilityOf(unit hero, bool withBonus)
        {
            return StatOf(hero, 1, withBonus);
        }

        /// <summary>The intelligence of a hero.</summary>
        public int IntelligenceOf(unit hero, bool withBonus)
        {
            return StatOf(hero, 2, withBonus);
        }

        /// <summary>Sets the base strength of a hero.</summary>
        public void SetStrength(unit hero, int strength)
        {
            SetStat(hero, 0, "strength", strength);
        }

        /// <summary>Sets the base agility of a hero.</summary>
        public void SetAgility(unit hero, int agility)
        {
            SetStat(hero, 1, "agility", agility);
        }

        /// <summary>Sets the base intelligence of a hero.</summary>
        public void SetIntelligence(unit hero, int intelligence)
        {
            SetStat(hero, 2, "intelligence", intelligence);
        }

        /// <summary>The skill points of a hero; 0 until set.</summary>
        public int SkillPointsOf(unit hero)
        {
            NoteGone(hero);
            return !vanishedUnits.Contains(hero) && skillPoints.TryGetValue(hero, out var points) ? points : 0;
        }

        /// <summary>Adds skill points to a hero and records "skill-points"; true unless the points would go below 0, which leaves them as they were.</summary>
        public bool AddSkillPoints(unit hero, int delta)
        {
            NoteGone(hero);
            HeroCalls.Add("skill-points " + delta);
            var now = SkillPointsOf(hero) + delta;
            if (now < 0)
            {
                return false;
            }

            skillPoints[hero] = now;
            return true;
        }

        /// <summary>Revives a dead hero at the place and records a "revive"; false for a hero that is not dead.</summary>
        public bool ReviveAt(unit hero, float x, float y, bool showEffect)
        {
            NoteGone(hero);
            HeroCalls.Add("revive " + (int)x + " " + (int)y + " " + Flag(showEffect));
            if (!dead.Remove(hero))
            {
                return false;
            }

            unitXs[hero] = x;
            unitYs[hero] = y;
            return true;
        }

        /// <summary>A flag as the hero calls record it: "True" or "False", the same on .NET and in Lua.</summary>
        private static string Flag(bool value)
        {
            return value ? "True" : "False";
        }

        /// <summary>Whether a unit is paused.</summary>
        public bool IsPausedUnit(unit unit)
        {
            NoteGone(unit);
            return !vanishedUnits.Contains(unit) && PausedUnits.Contains(unit);
        }

        /// <summary>Pauses or unpauses a unit.</summary>
        public void SetPaused(unit unit, bool paused)
        {
            NoteGone(unit);
            PausedUnits.Remove(unit);
            if (paused)
            {
                PausedUnits.Add(unit);
            }
        }

        /// <summary>The scale of a unit; 1 until set.</summary>
        public float ScaleOf(unit unit)
        {
            NoteGone(unit);
            if (vanishedUnits.Contains(unit))
            {
                return 0.0f;
            }

            return scales.TryGetValue(unit, out var scale) ? scale : 1.0f;
        }

        /// <summary>Sets the scale of a unit, and counts the call.</summary>
        public void ScaleUnit(unit unit, float scale)
        {
            NoteGone(unit);
            ScaleCalls++;
            scales[unit] = scale;
        }

        /// <summary>The name of a unit; "" until set.</summary>
        public string UnitNameOf(unit unit)
        {
            NoteGone(unit);
            return !vanishedUnits.Contains(unit) && unitNames.TryGetValue(unit, out var name) ? name : "";
        }

        /// <summary>Sets the name of a unit, and records a "rename".</summary>
        public void RenameUnit(unit unit, string name)
        {
            NoteGone(unit);
            UnitCalls.Add("rename");
            unitNames[unit] = name;
        }

        /// <summary>The proper name of a hero; "" until set.</summary>
        public string ProperNameOf(unit hero)
        {
            NoteGone(hero);
            return !vanishedUnits.Contains(hero) && properNames.TryGetValue(hero, out var name) ? name : "";
        }

        /// <summary>Sets the proper name of a hero, and records a "proper-name".</summary>
        public void SetProperName(unit hero, string name)
        {
            NoteGone(hero);
            HeroCalls.Add("proper-name " + name);
            properNames[hero] = name;
        }

        /// <summary>Keeps the four values a unit was tinted with.</summary>
        public void TintUnit(unit unit, int red, int green, int blue, int alpha)
        {
            NoteGone(unit);
            Tints[unit] = new int[] { red, green, blue, alpha };
        }

        /// <summary>The fly height of a unit; 0 until set.</summary>
        public float FlyHeightOf(unit unit)
        {
            return Kept(flyHeights, unit);
        }

        /// <summary>Whether the unit's type flies.</summary>
        public bool CanFly(unit unit)
        {
            NoteGone(unit);
            return flyers.Contains(unit);
        }

        /// <summary>Sets the fly height of a unit.</summary>
        public void SetFlyHeight(unit unit, float height)
        {
            NoteGone(unit);
            flyHeights[unit] = height;
        }

        /// <summary>Sets the number of inventory slots a unit has.</summary>
        public void SetInventorySize(unit unit, int size)
        {
            inventorySizes[unit] = size;
        }

        /// <summary>The slots of a unit's inventory, made on first use.</summary>
        private item[] SlotsOf(unit unit)
        {
            if (!inventories.TryGetValue(unit, out var slots))
            {
                slots = new item[InventorySizeOf(unit)];
                inventories[unit] = slots;
            }

            return slots;
        }

        /// <summary>Records an order and gives the answer set in OrderAnswer.</summary>
        public bool OrderUnit(unit unit, string order)
        {
            NoteGone(unit);
            Orders.Add("immediate " + order);
            return OrderAnswer;
        }

        /// <summary>Records an order to a place and gives the answer set in OrderAnswer.</summary>
        public bool OrderUnitAt(unit unit, string order, float x, float y)
        {
            NoteGone(unit);
            Orders.Add("point " + order);
            OrderPoints.Add(x);
            OrderPoints.Add(y);
            return OrderAnswer;
        }

        /// <summary>Records an order on a widget and gives the answer set in OrderAnswer.</summary>
        public bool OrderUnitOn(unit unit, string order, widget target)
        {
            NoteGone(unit);
            Orders.Add("target " + order);
            OrderTargets.Add(target);
            return OrderAnswer;
        }

        /// <summary>The number of inventory slots of a unit; 6 until set.</summary>
        public int InventorySizeOf(unit unit)
        {
            NoteGone(unit);
            if (vanishedUnits.Contains(unit))
            {
                return 0;
            }

            return inventorySizes.TryGetValue(unit, out var size) ? size : 6;
        }

        /// <summary>The item in a slot; null when empty or out of range.</summary>
        public item ItemInSlot(unit unit, int slot)
        {
            NoteGone(unit);
            ItemSlotReads++;
            var slots = SlotsOf(unit);
            return slot >= 0 && slot < slots.Length ? slots[slot] : null!;
        }

        /// <summary>Puts an item in the first free slot, and as the game does, delivers the pick-up when it is listened for; false, with no event, when no slot is free. An item the unit already holds keeps its slot and fires a drop and then a pick-up (seen 10-21).</summary>
        public bool GiveItem(unit unit, item item)
        {
            NoteGone(unit);
            var slots = SlotsOf(unit);
            for (var i = 0; i < slots.Length; i++)
            {
                if (slots[i] == item)
                {
                    InventoryCalls.Add("give");
                    if (Listening.Contains(GameEvent.Drop))
                    {
                        Inbound.Dropped(unit, item);
                    }

                    if (Listening.Contains(GameEvent.PickUp))
                    {
                        Inbound.PickedUp(unit, item);
                    }

                    return true;
                }
            }

            for (var i = 0; i < slots.Length; i++)
            {
                if (slots[i] == null)
                {
                    InventoryCalls.Add("give");
                    slots[i] = item;
                    if (Listening.Contains(GameEvent.PickUp))
                    {
                        Inbound.PickedUp(unit, item);
                    }

                    return true;
                }
            }

            return false;
        }

        /// <summary>Takes an item out of its slot and puts it at the unit's place; the game's drop event, when it is listened for, comes while the unit still has the item in the slot (seen 10-21).</summary>
        public void DropItem(unit unit, item item)
        {
            NoteGone(unit);
            var slots = SlotsOf(unit);
            for (var i = 0; i < slots.Length; i++)
            {
                if (slots[i] == item)
                {
                    InventoryCalls.Add("drop");
                    if (Listening.Contains(GameEvent.Drop))
                    {
                        Inbound.Dropped(unit, item);
                    }

                    if (slots[i] == item)
                    {
                        slots[i] = null!;
                    }

                    LandAt(unit, item);
                    return;
                }
            }
        }
    }
}
