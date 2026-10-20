using System.Collections.Generic;

namespace Moonwell.Hosting
{
    /// <summary>The game as the runtime sees it; the game implements it later, and the tests fake it.</summary>
    public interface IWorld
    {
        /// <summary>Game seconds since the runtime started.</summary>
        float Now();

        /// <summary>Calls Runtime.Wake() once after the seconds; replaces any earlier request.</summary>
        void WakeIn(float seconds);

        /// <summary>Starts delivering this event to Inbound; called once per event.</summary>
        void Listen(GameEvent gameEvent);

        /// <summary>Delivers this destructable's death to Inbound.</summary>
        void WatchDeath(destructable destructable);

        /// <summary>Shows a line on every player's screen.</summary>
        void Print(string text);

        /// <summary>Shows a line to one player, for the seconds, or for the game's own time when the seconds are below 0.</summary>
        void PrintTo(player player, string text, float seconds);

        /// <summary>Shows a line to every player, for the seconds, or for the game's own time when the seconds are below 0.</summary>
        void PrintToAll(string text, float seconds);

        /// <summary>Appends a line to CustomMapData\Moonwell\log.txt.</summary>
        void Log(string text);

        /// <summary>The player in a slot.</summary>
        player PlayerAt(int slot);

        /// <summary>The slot of a player.</summary>
        int SlotOf(player player);

        /// <summary>The slot of the neutral hostile player.</summary>
        int NeutralAggressiveSlot();

        /// <summary>The slot of the neutral victim player.</summary>
        int NeutralVictimSlot();

        /// <summary>The slot of the neutral extra player.</summary>
        int NeutralExtraSlot();

        /// <summary>The slot of the neutral passive player.</summary>
        int NeutralPassiveSlot();

        /// <summary>The x of a player's start location.</summary>
        float StartXOf(player player);

        /// <summary>The y of a player's start location.</summary>
        float StartYOf(player player);

        /// <summary>Whether a player is the one at this machine.</summary>
        bool IsLocal(player player);

        /// <summary>Every unit in the world.</summary>
        List<unit> UnitsInWorld();

        /// <summary>Every item in the world.</summary>
        List<item> ItemsInWorld();

        /// <summary>Every destructable in the world.</summary>
        List<destructable> DestructablesInWorld();

        /// <summary>Creates a unit of a type for an owner at a place and facing.</summary>
        unit SpawnUnit(player owner, int typeId, float x, float y, float facing);

        /// <summary>Creates an item of a type at a place.</summary>
        item SpawnItem(int typeId, float x, float y);

        /// <summary>The type id of a unit; 0 once the game has removed it.</summary>
        int UnitTypeId(unit unit);

        /// <summary>The type id of an item; 0 once the game has removed it.</summary>
        int ItemTypeId(item item);

        /// <summary>The type id of a destructable; 0 once the game has removed it.</summary>
        int DestructableTypeId(destructable destructable);

        /// <summary>Removes a unit from the world.</summary>
        void DeleteUnit(unit unit);

        /// <summary>Removes an item from the world.</summary>
        void DeleteItem(item item);

        /// <summary>Removes a destructable from the world.</summary>
        void DeleteDestructable(destructable destructable);

        /// <summary>Whether a unit is a hero.</summary>
        bool IsHero(unit unit);

        /// <summary>The player that owns a unit; null a while after the game has removed it.</summary>
        player OwnerOf(unit unit);

        /// <summary>The slot state of a player, as SlotState counts it (0 empty, 1 playing, 2 left).</summary>
        int SlotStateOf(player player);

        /// <summary>Who controls a player, as Controller counts it (0 user, 1 computer, 2 rescuable, 3 neutral, 4 creep, 5 none).</summary>
        int ControllerOf(player player);

        /// <summary>A state of a player: its gold, lumber, food or upkeep.</summary>
        int StateOf(player player, PlayerState state);

        /// <summary>Sets a state of a player.</summary>
        void SetState(player player, PlayerState state, int value);

        /// <summary>The level a player has of an upgrade; 0 when the player has none.</summary>
        int UpgradeLevel(player p, int upgradeId);

        /// <summary>Sets the level a player has of an upgrade.</summary>
        void Research(player player, int upgradeId, int level);

        /// <summary>How many of a unit type a player may have, or the top level of an upgrade it may research; int.MaxValue for a unit type without a limit.</summary>
        int TechMaxOf(player player, int typeId);

        /// <summary>Sets how many of a unit type a player may have, or the top level of an upgrade; 0 forbids it.</summary>
        void SetTechMax(player player, int typeId, int max);

        /// <summary>Makes an ability available to a player's units or not.</summary>
        void MakeAvailable(player player, int abilityId, bool available);

        /// <summary>The name of a player.</summary>
        string NameOf(player player);

        /// <summary>Sets the name of a player.</summary>
        void Rename(player player, string name);

        /// <summary>The colour of a player, as PlayerColor counts it.</summary>
        int ColorOf(player player);

        /// <summary>Sets the colour of a player and of the units it has.</summary>
        void Recolor(player player, int color);

        /// <summary>The team of a player.</summary>
        int TeamOf(player player);

        /// <summary>Sets the team of a player.</summary>
        void MoveToTeam(player player, int team);

        /// <summary>The race of a player, as Race counts it.</summary>
        int RaceOf(player player);

        /// <summary>What one player gives another, as Alliance's flags.</summary>
        int AllianceOf(player source, player other);

        /// <summary>Sets what one player gives another, as Alliance's flags; it goes one way.</summary>
        void SetAllianceOf(player source, player other, int alliance);

        /// <summary>Whether the game counts the other player as the player's ally.</summary>
        bool IsAllyOf(player source, player other);

        /// <summary>Whether the game counts the other player as the player's enemy.</summary>
        bool IsEnemyOf(player source, player other);

        /// <summary>The level of an ability on a unit; 0 when the unit has none.</summary>
        int AbilityLevel(unit unit, int abilityId);

        /// <summary>Gives a unit an ability.</summary>
        void AddAbility(unit unit, int abilityId);

        /// <summary>Hides an ability on a unit.</summary>
        void HideAbility(unit unit, int abilityId);

        /// <summary>Takes an ability from a unit.</summary>
        void RemoveAbility(unit unit, int abilityId);
    }
}
