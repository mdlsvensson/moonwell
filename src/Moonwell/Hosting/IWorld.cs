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

        /// <summary>The x of a destructable.</summary>
        float DestructableX(destructable destructable);

        /// <summary>The y of a destructable.</summary>
        float DestructableY(destructable destructable);

        /// <summary>Removes a unit from the world.</summary>
        void DeleteUnit(unit unit);

        /// <summary>Removes an item from the world.</summary>
        void DeleteItem(item item);

        /// <summary>Removes a destructable from the world.</summary>
        void DeleteDestructable(destructable destructable);

        /// <summary>Whether a unit is a hero.</summary>
        bool IsHero(unit unit);

        /// <summary>Whether a unit is an illusion, as Mirror Image and a Wand of Illusion make.</summary>
        bool IsIllusion(unit unit);

        /// <summary>The player that owns a unit; null a while after the game has removed it.</summary>
        player OwnerOf(unit unit);

        /// <summary>Sets the player that owns a unit, changing the unit's colour to the new player's.</summary>
        void SetOwner(unit unit, player player);

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

        /// <summary>The life of a unit.</summary>
        float LifeOf(unit unit);

        /// <summary>Sets the life of a unit; a unit left below 0.405 life dies.</summary>
        void SetLife(unit unit, float life);

        /// <summary>The most life a unit can have.</summary>
        float MaxLifeOf(unit unit);

        /// <summary>Sets the most life a unit can have, as a whole number; a life above it falls to it.</summary>
        void SetMaxLife(unit unit, float life);

        /// <summary>The mana of a unit.</summary>
        float ManaOf(unit unit);

        /// <summary>Sets the mana of a unit.</summary>
        void SetMana(unit unit, float mana);

        /// <summary>The most mana a unit can have.</summary>
        float MaxManaOf(unit unit);

        /// <summary>Sets the most mana a unit can have, as a whole number; a mana above it falls to it.</summary>
        void SetMaxMana(unit unit, float mana);

        /// <summary>The x of a unit.</summary>
        float UnitX(unit unit);

        /// <summary>The y of a unit.</summary>
        float UnitY(unit unit);

        /// <summary>Moves a unit to a place the way the game does for a unit that is told to stand there: it finds a free spot and the unit's order ends.</summary>
        void MoveUnit(unit unit, float x, float y);

        /// <summary>Puts a unit at a place with no pathing check; the unit's order goes on.</summary>
        void PlaceUnit(unit unit, float x, float y);

        /// <summary>The facing of a unit, in degrees.</summary>
        float FacingOf(unit unit);

        /// <summary>Orders a unit to turn to a facing, in degrees, at its turn rate.</summary>
        void TurnUnit(unit unit, float degrees);

        /// <summary>Sets the facing of a unit, in degrees, at once.</summary>
        void FaceUnitNow(unit unit, float degrees);

        /// <summary>The level of a unit; a hero's hero level.</summary>
        int LevelOf(unit unit);

        /// <summary>The name of a unit, in this machine's language; a hero's is its type's name, not its proper name.</summary>
        string UnitNameOf(unit unit);

        /// <summary>Sets the name of one unit.</summary>
        void RenameUnit(unit unit, string name);

        /// <summary>A hero's proper name.</summary>
        string ProperNameOf(unit hero);

        /// <summary>Sets a hero's proper name.</summary>
        void SetProperName(unit hero, string name);

        /// <summary>The value a type declares for a field, as the build wrote it for the map; null when it wrote none.</summary>
        object? TypeValue(int typeId, string field);

        /// <summary>Deals damage from a unit to another, with the game's attack, damage and weapon numbers; the damage event runs inside the call. Whether the game dealt it.</summary>
        bool DealDamage(unit source, unit target, float amount, bool attack, bool ranged, int attackType, int damageType, int weaponType);

        /// <summary>Whether a unit is alive.</summary>
        bool IsAlive(unit unit);

        /// <summary>Kills a unit through the game; the game then delivers the unit's death, with no killer, before the call returns.</summary>
        void SlayUnit(unit unit);

        /// <summary>A hero's experience.</summary>
        int ExperienceOf(unit hero);

        /// <summary>Sets a hero's experience; the game gives the levels that come with it, with the level-up effect when asked.</summary>
        void SetExperience(unit hero, int experience, bool showEffect);

        /// <summary>Gives a hero a level, with the level-up effect when asked.</summary>
        void LevelHero(unit hero, int level, bool showEffect);

        /// <summary>A hero's strength; with the bonus items and auras give when asked.</summary>
        int StrengthOf(unit hero, bool withBonus);

        /// <summary>A hero's agility; with the bonus items and auras give when asked.</summary>
        int AgilityOf(unit hero, bool withBonus);

        /// <summary>A hero's intelligence; with the bonus items and auras give when asked.</summary>
        int IntelligenceOf(unit hero, bool withBonus);

        /// <summary>Sets a hero's base strength for good.</summary>
        void SetStrength(unit hero, int strength);

        /// <summary>Sets a hero's base agility for good.</summary>
        void SetAgility(unit hero, int agility);

        /// <summary>Sets a hero's base intelligence for good.</summary>
        void SetIntelligence(unit hero, int intelligence);

        /// <summary>The skill points a hero has to spend.</summary>
        int SkillPointsOf(unit hero);

        /// <summary>Adds skill points to a hero, or takes them with a negative number; the game's answer.</summary>
        bool AddSkillPoints(unit hero, int delta);

        /// <summary>Revives a dead hero at a place, with the revive effect when asked; the game's answer.</summary>
        bool ReviveAt(unit hero, float x, float y, bool showEffect);

        /// <summary>Whether a unit is paused.</summary>
        bool IsPausedUnit(unit unit);

        /// <summary>Pauses or unpauses a unit.</summary>
        void SetPaused(unit unit, bool paused);

        /// <summary>The size of a unit's model, as a factor.</summary>
        float ScaleOf(unit unit);

        /// <summary>Sets the size of a unit's model on all three axes.</summary>
        void ScaleUnit(unit unit, float scale);

        /// <summary>Tints a unit's model; each value is 0-255.</summary>
        void TintUnit(unit unit, int red, int green, int blue, int alpha);

        /// <summary>The height a unit flies at above the ground.</summary>
        float FlyHeightOf(unit unit);

        /// <summary>Whether the unit's type flies.</summary>
        bool CanFly(unit unit);

        /// <summary>Sets the height a unit flies at, at once.</summary>
        void SetFlyHeight(unit unit, float height);

        /// <summary>Gives a unit an order that needs no place and no target; the game's answer.</summary>
        bool OrderUnit(unit unit, string order);

        /// <summary>Gives a unit an order to a place; the game's answer.</summary>
        bool OrderUnitAt(unit unit, string order, float x, float y);

        /// <summary>Gives a unit an order on a widget (a unit, an item or a destructable); the game's answer.</summary>
        bool OrderUnitOn(unit unit, string order, widget target);

        /// <summary>The number of inventory slots of a unit.</summary>
        int InventorySizeOf(unit unit);

        /// <summary>The item in a slot of a unit's inventory (0-based); null when the slot is empty.</summary>
        item ItemInSlot(unit unit, int slot);

        /// <summary>Gives a unit an item to pick up into its inventory, which the game counts as a pick-up; the game's answer.</summary>
        bool GiveItem(unit unit, item item);

        /// <summary>Makes a unit drop an item from its inventory, which the game counts as a drop.</summary>
        void DropItem(unit unit, item item);

        /// <summary>Whether a unit has an item in its inventory.</summary>
        bool CarriesItem(unit unit, item item);

        /// <summary>The number of charges of an item.</summary>
        int ChargesOf(item item);

        /// <summary>Sets the number of charges of an item.</summary>
        void SetCharges(item item, int charges);

        /// <summary>The player that owns an item.</summary>
        player ItemOwnerOf(item item);

        /// <summary>Sets the player that owns an item, leaving the item's colour as it is.</summary>
        void SetItemOwner(item item, player player);

        /// <summary>The x of an item.</summary>
        float ItemX(item item);

        /// <summary>The y of an item.</summary>
        float ItemY(item item);

        /// <summary>Moves an item to a place.</summary>
        void MoveItem(item item, float x, float y);
    }
}
