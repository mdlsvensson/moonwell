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

        /// <summary>The slot of the player at this machine.</summary>
        public int LocalSlot;

        /// <summary>The slots the game gives the neutral players.</summary>
        public int NeutralAggressive = 24, NeutralVictim = 25, NeutralExtra = 26, NeutralPassive = 27;

        /// <summary>The lines printed on the screen.</summary>
        public List<string> Printed = new List<string>();

        /// <summary>The slot of each line printed to a player, -1 for a line printed to everyone.</summary>
        public List<int> PrintedSlots = new List<int>();

        /// <summary>Each line printed to a player or to everyone.</summary>
        public List<string> PrintedTexts = new List<string>();

        /// <summary>The seconds of each line printed to a player or to everyone; below 0 for the game's own time.</summary>
        public List<float> PrintedSeconds = new List<float>();

        /// <summary>The lines appended to the log.</summary>
        public List<string> Logged = new List<string>();

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

        /// <summary>The ability levels of each unit, keyed by ability id.</summary>
        private readonly Dictionary<unit, Dictionary<int, int>> abilityLevels = new Dictionary<unit, Dictionary<int, int>>();

        /// <summary>The type id of each item the world made.</summary>
        private readonly Dictionary<item, int> itemTypes = new Dictionary<item, int>();

        /// <summary>The type id of each destructable the world made.</summary>
        private readonly Dictionary<destructable, int> destructableTypes = new Dictionary<destructable, int>();

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

        /// <summary>The slot of each player made.</summary>
        private readonly Dictionary<player, int> slots = new Dictionary<player, int>();

        /// <summary>Makes a fresh world for one case and makes it the runtime's world.</summary>
        public static FakeWorld Install()
        {
            var world = new FakeWorld();
            Runtime.Reset(world);
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

        /// <summary>Makes a unit vanish the way the game does: its type id and its owner read 0 and null, and nothing is told.</summary>
        public void VanishUnit(unit u)
        {
            unitTypes[u] = 0;
            ownerless.Add(u);
        }

        /// <summary>Makes an item vanish the way the game does: its type id reads 0 and nothing is told.</summary>
        public void VanishItem(item i)
        {
            itemTypes[i] = 0;
        }

        /// <summary>Makes a destructable vanish the way the game does: its type id reads 0 and nothing is told.</summary>
        public void VanishDestructable(destructable d)
        {
            destructableTypes[d] = 0;
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

        /// <summary>Sets the start location of a player slot.</summary>
        public void SetStart(int slot, float x, float y)
        {
            startXs[slot] = x;
            startYs[slot] = y;
        }

        /// <summary>Moves time forward by the seconds, calling Runtime.Wake at each wake request on the way.</summary>
        public void Advance(float seconds)
        {
            var target = Time + seconds;
            while (WakeAt.HasValue && WakeAt.Value <= target + 0.001f)
            {
                Time = WakeAt.Value;
                WakeAt = null;
                Runtime.Wake();
            }

            Time = target;
        }

        /// <summary>The game seconds since the world was made.</summary>
        public float Now()
        {
            return Time;
        }

        /// <summary>Requests a wake the seconds from now; replaces any earlier request.</summary>
        public void WakeIn(float seconds)
        {
            WakeAt = Time + seconds;
        }

        /// <summary>Records that the runtime asked to hear an event, once.</summary>
        public void Listen(GameEvent gameEvent)
        {
            if (!Listening.Contains(gameEvent))
            {
                Listening.Add(gameEvent);
            }
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

        /// <summary>Makes a unit owned by the owner's slot; the place and facing are not kept.</summary>
        public unit SpawnUnit(player owner, int typeId, float x, float y, float facing)
        {
            return NewUnit(typeId, SlotOf(owner));
        }

        /// <summary>Makes an item; the place is not kept.</summary>
        public item SpawnItem(int typeId, float x, float y)
        {
            return NewItem(typeId);
        }

        /// <summary>The type id of a unit; 0 once it is removed or vanished.</summary>
        public int UnitTypeId(unit unit)
        {
            if (unitTypes.TryGetValue(unit, out var typeId))
            {
                return typeId;
            }

            return 0;
        }

        /// <summary>The type id of an item; 0 once it is removed or vanished.</summary>
        public int ItemTypeId(item item)
        {
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

        /// <summary>Records a unit removed by the runtime, and reads its type id as 0.</summary>
        public void DeleteUnit(unit unit)
        {
            RemovedUnits.Add(unit);
            unitTypes[unit] = 0;
        }

        /// <summary>Records an item removed by the runtime, and reads its type id as 0.</summary>
        public void DeleteItem(item item)
        {
            RemovedItems.Add(item);
            itemTypes[item] = 0;
        }

        /// <summary>Records a destructable removed by the runtime, and reads its type id as 0.</summary>
        public void DeleteDestructable(destructable destructable)
        {
            RemovedDestructables.Add(destructable);
            destructableTypes[destructable] = 0;
        }

        /// <summary>Whether a unit was made as a hero.</summary>
        public bool IsHero(unit unit)
        {
            return heroes.TryGetValue(unit, out var hero) && hero;
        }

        /// <summary>The player owning a unit; null for a vanished unit, slot 0 for a unit the world did not make.</summary>
        public player OwnerOf(unit unit)
        {
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

        /// <summary>Records an add-ability call.</summary>
        public void AddAbility(unit unit, int abilityId)
        {
            AbilityCalls.Add("add " + abilityId);
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
        }
    }
}
