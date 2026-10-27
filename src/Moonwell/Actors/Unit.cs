using System;
using System.Collections.Generic;
using System.ComponentModel;

namespace Moonwell
{
    /// <summary>A unit of the game: an object of its declared class, which holds the components the class starts with.</summary>
    public class Unit : Widget
    {
        /// <summary>The table behind UnitHooks, made on first use. A static initializer would not do: CSharp.lua runs it when the class is first indexed, and a map's static Dictionary keyed by Unit does that while the script loads, before this file has its imports.</summary>
        private static HookTable? unitHooks;

        /// <summary>The reactions of units; declared here, so that an override in a subclass is found.</summary>
        internal static HookTable UnitHooks
        {
            get
            {
                if (unitHooks == null)
                {
                    unitHooks = new HookTable(typeof(Unit), new string[] { "OnItemPickedUp", "OnItemDropped", "OnDamaged", "OnDealtDamage", "OnDeath", "OnKill", "OnCast" }, new int[] { Hook.OnItemPickedUp, Hook.OnItemDropped, Hook.OnDamaged, Hook.OnDealtDamage, Hook.OnDeath, Hook.OnKill, Hook.OnCast });
                }

                return unitHooks;
            }
        }

        /// <summary>The bit mask of the reactions this class overrides, worked out when the unit is taken in.</summary>
        internal int Mask;

        /// <summary>The cast the unit is making now, which ends when its spell ends or its next cast begins; null when none.</summary>
        internal Cast? ActiveCast;

        /// <summary>The components the class starts with, added in order when the unit enters. Assigning a new list drops what a parent class listed.</summary>
        protected List<UnitComponent> Components { get; set; } = new List<UnitComponent>();

        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same (see Owner).

        /// <summary>The ability objects of the unit, one per ability id, in the order they were first asked for.</summary>
        internal readonly OrderedMap<int, Ability> abilities = new OrderedMap<int, Ability>();

        /// <summary>The game's handle of the unit.</summary>
        [LocalSafe]
        public unit Handle { get; internal set; } = null!;

        /// <summary>The type id of the unit, as the game read it when the unit was taken in.</summary>
        [LocalSafe]
        public int TypeId { get; internal set; }

        /// <summary>Whether the unit is an illusion (Mirror Image, a Wand of Illusion), as the game read it when the unit was taken in. An illusion has its hero's type id but is taken in as a plain unit, not as the hero's class: it has none of the class's starting components and runs no OnCreated. Map-wide reactions (OnUnitCreated, OnUnitDied) still see it.</summary>
        [LocalSafe]
        public bool IsIllusion { get; internal set; }

        /// <summary>The default object of the unit's type, as Unreal's class default object: the values its class declares (a type no class of the map's declares, its values as World Editor made them, standard where World Editor left them), the same whatever this unit's are now and once it is gone. One per type, shared by its units, with no handle; it is gone, so its live actions do nothing.</summary>
        [LocalSafe]
        public Unit Default => ObjectTypes.DefaultOf(this, DeclaredTypeId);

        /// <summary>The id of the type whose declared values the unit reads: its type id, else, while its constructor runs, the id its class is registered with, else 0.</summary>
        internal int DeclaredTypeId => TypeId != 0 ? TypeId : ObjectTypes.IdOfClass(this);

        /// <summary>The value the unit's type declares for a field, which a shared member reads when the unit has no handle (in its constructor, and on a default object). Here, the value the build wrote for the map; the standard classes read their own declared values on .NET.</summary>
        [EditorBrowsable(EditorBrowsableState.Never)]
        protected virtual T DeclaredValue<T>(string field)
        {
            // The build has no world; the classes it reads override this.
            if (Runtime.World == null)
            {
                return default(T)!;
            }

            var value = Runtime.World.TypeValue(DeclaredTypeId, field);
            return value == null ? default(T)! : (T)value;
        }

        /// <summary>Declares the value of a field for the unit's class, which a shared member does when it is set with no handle. Here, nothing: in the game the values are the build's; the standard classes keep them on .NET.</summary>
        [EditorBrowsable(EditorBrowsableState.Never)]
        protected virtual void Declare(string field, object value)
        {
        }

        /// <summary>The game's player that owned the unit when it was last read; kept for when the game no longer gives it.</summary>
        internal player owner = null!;

        /// <summary>The player that owns the unit. Setting it takes effect at once and gives the unit the new player's colour (SetUnitOwner). Once the game has removed the unit, the last owner read or set, and setting does nothing.</summary>
        [LocalSafe]
        public Player Player
        {
            get
            {
                if (!IsGone)
                {
                    var now = Runtime.World.OwnerOf(Handle);
                    if (now != null)
                    {
                        if (Local.Depth != 0)
                        {
                            return Player.Of(now)!;
                        }

                        owner = now;
                    }
                }

                return Player.Of(owner)!;
            }

            set
            {
                if (!IsGone)
                {
                    owner = value.Handle;
                    Runtime.World.SetOwner(Handle, value.Handle);
                }
            }
        }

        /// <summary>The life last read, kept for when the game no longer gives it.</summary>
        internal float lastLife = 0.0f;

        /// <summary>The most life last read.</summary>
        internal float lastMaxLife = 0.0f;

        /// <summary>The mana last read.</summary>
        internal float lastMana = 0.0f;

        /// <summary>The most mana last read.</summary>
        internal float lastMaxMana = 0.0f;

        /// <summary>The x last read. Internal, as the interactions package reads a unit's place through the natives and keeps it here, as X does.</summary>
        internal float lastX = 0.0f;

        /// <summary>The y last read. Internal, as the interactions package reads a unit's place through the natives and keeps it here, as Y does.</summary>
        internal float lastY = 0.0f;

        /// <summary>The facing last read.</summary>
        internal float lastFacing = 0.0f;

        /// <summary>The level last read.</summary>
        internal int lastLevel;

        /// <summary>Whether the unit is paused, as last read or set.</summary>
        internal bool lastPaused;

        /// <summary>The scale last read or set.</summary>
        internal float lastScale = 1.0f;

        /// <summary>Whether the scale has been read or set, so that lastScale holds it.</summary>
        internal bool scaleKnown;

        /// <summary>The fly height last read or set.</summary>
        internal float lastFlyHeight = 0.0f;

        /// <summary>The inventory size last read.</summary>
        internal int lastInventorySize;

        /// <summary>The name last set.</summary>
        internal string lastName = "";

        /// <summary>Whether the unit can show a fly height: it flies, or FlyHeight has done the Amrf trick on it.</summary>
        internal bool flyReady;

        /// <summary>Whether a getter that read 0 read it from a unit the game has removed: the game gives 0 for everything then, and the type id 0, before the sweep finds the unit. The getter then keeps the last value instead of the 0.</summary>
        internal bool RemovedByGame()
        {
            return Runtime.World.UnitTypeId(Handle) == 0;
        }

        /// <summary>The unit's life; a value above the most life is clamped to it, and setting it to 0.405 or less kills the unit (0.406 does not) (the death event runs inside the call, with life 0). Once the game has removed the unit, the last life read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Life
        {
            get
            {
                if (IsGone)
                {
                    return lastLife;
                }

                var now = Runtime.World.LifeOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastLife;
                }

                if (Local.Depth == 0)
                {
                    lastLife = now;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    // The game clamps to the most life, so that is what a removed unit gives back.
                    var most = MaxLife;
                    lastLife = value <= 0.405f ? 0.0f : (value < most ? value : most);
                    Runtime.World.SetLife(Handle, value);
                }
            }
        }

        /// <summary>The most life the unit can have, a whole number (BlzSetUnitMaxHP; SetUnitState does not change it). The life stays unless the new most is below it, and then falls to it. Once the game has removed the unit, the last value read or set, and setting does nothing. In a declared class's constructor, the most life every unit of the class starts with (uhpm); on a default object, its type's.</summary>
        [LocalSafe]
        public float MaxLife
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("uhpm");
                }

                if (IsGone)
                {
                    return lastMaxLife;
                }

                var now = Runtime.World.MaxLifeOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastMaxLife;
                }

                if (Local.Depth == 0)
                {
                    lastMaxLife = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("uhpm", (int)value);
                    return;
                }

                if (!IsGone)
                {
                    // The game keeps the whole number, and lowers the life to it.
                    lastMaxLife = (int)value;
                    if (lastLife > lastMaxLife)
                    {
                        lastLife = lastMaxLife;
                    }

                    Runtime.World.SetMaxLife(Handle, value);
                }
            }
        }

        /// <summary>The unit's mana; a value above the most mana is clamped to it, and a unit without mana reads 0 and ignores a set. Once the game has removed the unit, the last mana read or set, and setting does nothing. In a declared class's constructor, the mana every unit of the class starts with (umpi); on a default object, its type's.</summary>
        [LocalSafe]
        public float Mana
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("umpi");
                }

                if (IsGone)
                {
                    return lastMana;
                }

                var now = Runtime.World.ManaOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastMana;
                }

                if (Local.Depth == 0)
                {
                    lastMana = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("umpi", (int)value);
                    return;
                }

                if (!IsGone)
                {
                    // The game clamps to the most mana (0 for a unit without mana), so that is what a removed unit gives back.
                    var most = MaxMana;
                    lastMana = value < most ? value : most;
                    if (lastMana < 0.0f)
                    {
                        lastMana = 0.0f;
                    }

                    Runtime.World.SetMana(Handle, value);
                }
            }
        }

        /// <summary>The most mana the unit can have, a whole number (BlzSetUnitMaxMana; SetUnitState does not change it). The mana stays unless the new most is below it, and then falls to it. Once the game has removed the unit, the last value read or set, and setting does nothing. In a declared class's constructor, the most mana every unit of the class starts with (umpm); on a default object, its type's.</summary>
        [LocalSafe]
        public float MaxMana
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("umpm");
                }

                if (IsGone)
                {
                    return lastMaxMana;
                }

                var now = Runtime.World.MaxManaOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastMaxMana;
                }

                if (Local.Depth == 0)
                {
                    lastMaxMana = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("umpm", (int)value);
                    return;
                }

                if (!IsGone)
                {
                    // The game keeps the whole number, and lowers the mana to it.
                    lastMaxMana = (int)value;
                    if (lastMana > lastMaxMana)
                    {
                        lastMana = lastMaxMana;
                    }

                    Runtime.World.SetMaxMana(Handle, value);
                }
            }
        }

        /// <summary>The x of the unit; once the game has removed it, the last x read.</summary>
        [LocalSafe]
        public float X
        {
            get
            {
                if (IsGone)
                {
                    return lastX;
                }

                var now = Runtime.World.UnitX(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastX;
                }

                if (Local.Depth == 0)
                {
                    lastX = now;
                }

                return now;
            }
        }

        /// <summary>The y of the unit; once the game has removed it, the last y read.</summary>
        [LocalSafe]
        public float Y
        {
            get
            {
                if (IsGone)
                {
                    return lastY;
                }

                var now = Runtime.World.UnitY(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastY;
                }

                if (Local.Depth == 0)
                {
                    lastY = now;
                }

                return now;
            }
        }

        /// <summary>Moves the unit to a place through the game's pathing (SetUnitPosition): the game finds a free spot, and the unit's order ends. Does nothing once the unit is gone.</summary>
        public void MoveTo(float x, float y)
        {
            if (!IsGone)
            {
                Runtime.World.MoveUnit(Handle, x, y);
            }
        }

        /// <summary>Puts the unit at a place with no pathing and no bounds (SetUnitX and SetUnitY): it may stand where it could not walk, or past the edge of the world, and its order goes on. Keep it inside the map yourself. Does nothing once the unit is gone.</summary>
        public void PlaceAt(float x, float y)
        {
            if (!IsGone)
            {
                Runtime.World.PlaceUnit(Handle, x, y);
            }
        }

        /// <summary>The unit's facing in degrees, within 0 to 360 (400 reads 40, -90 reads 270). Setting it makes the unit turn at its turn rate (SetUnitFacing), so a read just after still gives the old facing; FaceNow turns at once. Once the game has removed the unit, the last facing read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Facing
        {
            get
            {
                if (IsGone)
                {
                    return lastFacing;
                }

                var now = Runtime.World.FacingOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastFacing;
                }

                if (Local.Depth == 0)
                {
                    lastFacing = now;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    lastFacing = value;
                    Runtime.World.TurnUnit(Handle, value);
                }
            }
        }

        /// <summary>Turns the unit to a facing in degrees at once (BlzSetUnitFacingEx), so a read just after gives it. A unit that is walking turns back toward where it walks, and its order goes on. Does nothing once the unit is gone.</summary>
        public void FaceNow(float degrees)
        {
            if (!IsGone)
            {
                Runtime.World.FaceUnitNow(Handle, degrees);
            }
        }

        /// <summary>The unit's level; a hero's hero level. Once the game has removed the unit, the last level read. The game has no setter: a set in a declared class's constructor is the level every unit of the class has (ulev), and one on a unit in the game does nothing (the build rejects it). A hero's level is not declared: it starts at 1, which a hero with no handle reads. On a default object, its type's.</summary>
        [LocalSafe]
        public int Level
        {
            get
            {
                if (Handle == null)
                {
                    return this is Hero ? 1 : DeclaredValue<int>("ulev");
                }

                if (IsGone)
                {
                    return lastLevel;
                }

                var now = Runtime.World.LevelOf(Handle);
                if (now == 0 && RemovedByGame())
                {
                    return lastLevel;
                }

                if (Local.Depth == 0)
                {
                    lastLevel = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("ulev", value);
                }
            }
        }

        /// <summary>The unit's name (GetUnitName): a hero's is its type's name, not its proper name (seen 10-21). The game gives it in each player's language, so it is read only inside Local.For. Setting it (BlzSetUnitName) renames this unit only, the same on every machine. Once the game has removed the unit, the last name set (a read keeps nothing, as it differs by machine), and setting does nothing. In a declared class's constructor, the name of the class's type (unam), the same on every machine; on a default object, its type's, empty for a type the build's table does not hold.</summary>
        public string Name
        {
            [MachineLocal]
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<string>("unam") ?? string.Empty;
                }

                if (IsGone)
                {
                    return lastName;
                }

                var now = Runtime.World.UnitNameOf(Handle);
                if (string.IsNullOrEmpty(now) && RemovedByGame())
                {
                    return lastName;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("unam", value);
                    return;
                }

                if (!IsGone)
                {
                    lastName = value;
                    Runtime.World.RenameUnit(Handle, value);
                }
            }
        }

        /// <summary>Whether the unit is alive; false once the game has removed the unit.</summary>
        [LocalSafe]
        public bool IsAlive => !IsGone && Runtime.World.IsAlive(Handle);

        /// <summary>Kills the unit through the game (KillUnit): the game delivers the death as for any other, with no killer, so OnDeath runs once, inside this call. Does nothing once the unit is gone.</summary>
        public void Kill()
        {
            if (!IsGone)
            {
                Runtime.World.SlayUnit(Handle);

                // The game leaves the life 0 (seen), so a unit that is killed and then removed reads 0, not the life it had;
                // read it, as a hero may have been revived inside the death event.
                lastLife = Life;
            }
        }

        /// <summary>Whether the unit is paused (PauseUnit), a flag and not a count. A paused unit takes no orders (they answer false) and stays where it is, but its stand animation goes on. Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public bool IsPaused
        {
            get
            {
                if (IsGone)
                {
                    return lastPaused;
                }

                var now = Runtime.World.IsPausedUnit(Handle);
                if (!now && RemovedByGame())
                {
                    return lastPaused;
                }

                if (Local.Depth == 0)
                {
                    lastPaused = now;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    lastPaused = value;
                    Runtime.World.SetPaused(Handle, value);
                }
            }
        }

        /// <summary>The size of the unit's model as a factor of its normal size, set on all three axes (SetUnitScale). Until it is set, it reads the scale of the unit's type (0.8 for a footman), which is not 1. Once the game has removed the unit, the last value read or set, and setting does nothing. In a declared class's constructor, the scale every unit of the class starts with (usca); on a default object, its type's.</summary>
        [LocalSafe]
        public float Scale
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<float>("usca");
                }

                if (IsGone || scaleKnown)
                {
                    return lastScale;
                }

                var now = Runtime.World.ScaleOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastScale;
                }

                if (Local.Depth == 0)
                {
                    lastScale = now;
                    scaleKnown = true;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("usca", value);
                    return;
                }

                if (!IsGone)
                {
                    lastScale = value;
                    scaleKnown = true;
                    Runtime.World.ScaleUnit(Handle, value);
                }
            }
        }

        /// <summary>Tints the unit's model (SetUnitVertexColor); each value is clamped to 0-255, and an alpha of 0 makes the unit and its shadow invisible, though its selection circle and health bar still show. The game keeps no tint to read back. Does nothing once the unit is gone.</summary>
        public void Tint(int red, int green, int blue, int alpha = 255)
        {
            if (!IsGone)
            {
                Runtime.World.TintUnit(Handle, red, green, blue, alpha);
            }
        }

        /// <summary>How high the unit flies above the ground, set at once (SetUnitFlyHeight with a rate of 0). A unit that does not fly only shows the height once Amrf has been added to it and removed (seen 10-21), so the first set on such a unit does that; reading gives the height set either way. Once the game has removed the unit, the last value read or set, and setting does nothing. In a declared class's constructor, the height every unit of the class flies at (umvh); on a default object, its type's.</summary>
        [LocalSafe]
        public float FlyHeight
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<float>("umvh");
                }

                if (IsGone)
                {
                    return lastFlyHeight;
                }

                var now = Runtime.World.FlyHeightOf(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastFlyHeight;
                }

                if (Local.Depth == 0)
                {
                    lastFlyHeight = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("umvh", value);
                    return;
                }

                if (!IsGone)
                {
                    lastFlyHeight = value;
                    if (!flyReady)
                    {
                        flyReady = true;
                        // A unit that flies shows the height as it is; one that has Amrf must not lose it to the remove.
                        if (!Runtime.World.CanFly(Handle))
                        {
                            var crowForm = ObjectTypes.FourCC("Amrf");
                            if (Runtime.World.AbilityLevel(Handle, crowForm) <= 0)
                            {
                                Runtime.World.AddAbility(Handle, crowForm);
                                Runtime.World.RemoveAbility(Handle, crowForm);
                            }
                        }
                    }

                    Runtime.World.SetFlyHeight(Handle, value);
                }
            }
        }

        /// <summary>Gives the unit an order with no place and no target, by the game's order string ("stop", "holdposition"), and returns the game's answer. False, and no call to the game, once the unit is gone.</summary>
        public bool Order(string order)
        {
            return !IsGone && Runtime.World.OrderUnit(Handle, order);
        }

        /// <summary>Gives the unit an order to a place, by the game's order string ("move", "attack", "smart"), and returns the game's answer. False, and no call to the game, once the unit is gone.</summary>
        public bool OrderAt(string order, float x, float y)
        {
            return !IsGone && Runtime.World.OrderUnitAt(Handle, order, x, y);
        }

        /// <summary>Gives the unit an order on a unit, an item or a destructable, by the game's order string ("attack", "smart"), and returns the game's answer. False, and no call to the game, once the unit or the target is gone.</summary>
        public bool OrderOn(string order, Widget target)
        {
            if (IsGone || target.IsGone)
            {
                return false;
            }

            var unit = target as Unit;
            if (unit != null)
            {
                return Runtime.World.OrderUnitOn(Handle, order, unit.Handle);
            }

            var item = target as Item;
            if (item != null)
            {
                return Runtime.World.OrderUnitOn(Handle, order, item.Handle);
            }

            var destructable = target as Destructable;
            if (destructable != null)
            {
                return Runtime.World.OrderUnitOn(Handle, order, destructable.Handle);
            }

            return false;
        }

        /// <summary>Orders the unit to stop. Does nothing once the unit is gone.</summary>
        public void Stop()
        {
            Order("stop");
        }

        /// <summary>The number of inventory slots of the unit. Once the game has removed the unit, the last size read.</summary>
        [LocalSafe]
        public int InventorySize
        {
            get
            {
                if (IsGone)
                {
                    return lastInventorySize;
                }

                var now = Runtime.World.InventorySizeOf(Handle);
                if (now == 0 && RemovedByGame())
                {
                    return lastInventorySize;
                }

                if (Local.Depth == 0)
                {
                    lastInventorySize = now;
                }

                return now;
            }
        }

        /// <summary>The item in a slot of the unit's inventory, counting from 0; null when the slot is empty, when there is no such slot, and once the unit is gone. Not for Local.For code: an item Moonwell has not seen is taken in (its components and OnCreated run), which must happen on every machine.</summary>
        public Item? ItemIn(int slot)
        {
            if (IsGone || slot < 0 || slot >= InventorySize)
            {
                return null;
            }

            return Item.Of(Runtime.World.ItemInSlot(Handle, slot));
        }

        /// <summary>Gives the unit an item to pick up into its first free slot (a slot a drop freed is filled first), and returns the game's answer. The game counts it as a pick-up, so OnPickedUp and OnItemPickedUp run, inside this call. The answer is false, with no event and the item left where it is, when the inventory is full or the unit has none. Giving an item the unit already holds keeps its slot and runs the drop reactions and then the pick-up reactions (seen; the answer it gives was not recorded). False, and no call to the game, once the unit or the item is gone.</summary>
        public bool Give(Item item)
        {
            return !IsGone && !item.IsGone && Runtime.World.GiveItem(Handle, item.Handle);
        }

        /// <summary>Makes the unit drop an item from its inventory where it stands (UnitRemoveItem). The game counts it as a drop, so OnDropped and OnItemDropped run, inside this call and while the unit still has the item: the item's Carrier is the unit there, and null after the call. Does nothing for an item that lies on the ground, and once the unit or the item is gone. A unit that dies drops its items (a hero keeps them) with no drop reactions, and the items of a unit removed with RemoveUnit leave it inside that call, but the game fires their drop events after the call: Carrier is already null then, and the item's and the unit's drop reactions run for an object that is about to be swept (a unit that Remove() has already ended runs none). Seen for a hero, whose items are then removed with it; other units are assumed to do the same.</summary>
        public void Drop(Item item)
        {
            if (!IsGone && !item.IsGone)
            {
                Runtime.World.DropItem(Handle, item.Handle);
            }
        }

        /// <summary>The starting components, which the runtime adds in order when the unit enters.</summary>
        internal List<UnitComponent> StartComponents => Components;

        /// <summary>The object of a game unit: made when the unit is first seen; null for null or for a unit the game has removed.</summary>
        public static Unit? Of(unit? handle)
        {
            if (handle == null)
            {
                return null;
            }

            return KnownUnits.TakeIn(handle, null);
        }

        /// <summary>The facing of a unit made without one, in degrees: the game's bj_UNIT_FACING.</summary>
        public const float DefaultFacing = 270.0f;

        /// <summary>Creates a unit of a type for a player at a place and facing (270 degrees without one); owner, when given, owns it.</summary>
        public static Unit Create(int typeId, Player player, float x, float y, float facing = DefaultFacing, Owner? owner = null)
        {
            var handle = Runtime.World.SpawnUnit(player.Handle, typeId, x, y, facing);
            var made = KnownUnits.TakeIn(handle, owner)!;
            if (owner != null && made.Owner == null)
            {
                owner.Adopt(made);
            }

            return made;
        }

        /// <summary>Creates a unit of a registered class for a player at a place and facing (270 degrees without one); owner, when given, owns it.</summary>
        public static T Create<T>(Player player, float x, float y, float facing = DefaultFacing, Owner? owner = null)
            where T : Unit
        {
            return (T)Create(ObjectTypes.IdOf<T>(), player, x, y, facing, owner);
        }

        /// <summary>The ability of the unit with an id: one object per id, made by the registered maker or as a plain Ability, and owned by the unit.</summary>
        public Ability AbilityOf(int abilityId)
        {
            if (abilities.TryGetValue(abilityId, out var known))
            {
                return known;
            }

            var maker = ObjectTypes.AbilityMaker(abilityId);
            var made = maker != null ? maker() : new Ability();
            made.Unit = this;
            made.Id = abilityId;
            Adopt(made);
            made.Mask = Hooks.MaskOf(made, Ability.AbilityHooks);
            Events.Need(made.Mask);
            abilities[abilityId] = made;
            return made;
        }

        /// <summary>The ability of the unit of a registered class: the object of that class.</summary>
        public T AbilityOf<T>()
            where T : Ability => (T)AbilityOf(ObjectTypes.IdOf<T>());

        /// <summary>Adds a component to the unit, and returns it.</summary>
        public T Add<T>(T component)
            where T : UnitComponent => AddComponent(component);

        /// <summary>Applies a buff to the unit and returns the buff now on it: a buff of its class already there is refreshed, as its stacking says (it takes the given buff's duration and the source, and keeps its other values and what it has counted), and the given buff is dropped; otherwise the given buff is added and started.</summary>
        public T Apply<T>(T buff, Unit? source = null)
            where T : Buff
        {
            Dev.Check(buff.Duration >= 0.0f && buff.Tick >= 0.0f, "Unit.Apply: Duration and Tick must be 0 or more");
            var existing = StackedWith(buff, source);
            if (existing != null)
            {
                existing.Refresh(buff, source);
                return (T)existing;
            }

            buff.Source = source;
            Add(buff);
            if (!buff.HasEnded)
            {
                buff.Begin();
            }

            return buff;
        }

        /// <summary>The buff on the unit that a new apply stacks with, or null: the first of its class for Refresh, the first of its class from the same source for PerSource, none for Separate.</summary>
        private Buff? StackedWith(Buff buff, Unit? source)
        {
            if (buff.Stacking == Stacking.Separate)
            {
                return null;
            }

            var found = GetAll<Buff>();
            var key = Names.ClassOf(buff);
            for (var i = 0; i < found.Count; i++)
            {
                var other = found[i];
                if (other.HasEnded || Names.ClassOf(other) != key)
                {
                    continue;
                }

                if (buff.Stacking == Stacking.Refresh || other.Source == source)
                {
                    return other;
                }
            }

            return null;
        }

        /// <summary>Removes the unit from the game: it ends, which runs its OnLeave. Does nothing on a default object, which is not in the game.</summary>
        public void Remove()
        {
            if (Handle != null)
            {
                End();
            }
        }

        /// <summary>Runs once, when the unit comes into the game, after its starting components are added.</summary>
        protected virtual void OnCreated()
        {
        }

        /// <summary>Runs when the unit leaves the game, before its components end; HasEnded is still false here.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Runs OnCreated; Units calls it, since a map overrides the hook as protected.</summary>
        internal void CallEnter()
        {
            OnCreated();
        }

        /// <summary>Runs when the unit picks up an item, before the unit's components see it.</summary>
        /// <param name="pickedItem">The item the unit picked up.</param>
        public virtual void OnItemPickedUp(Item pickedItem)
        {
        }

        /// <summary>Runs when the unit drops an item, before the unit's components see it. Not run for the items a dying unit drops; see Drop.</summary>
        /// <param name="droppedItem">The item the unit dropped.</param>
        public virtual void OnItemDropped(Item droppedItem)
        {
        }

        /// <summary>Runs when the unit takes damage, before the unit's components see it.</summary>
        public virtual void OnDamaged(Damage damage)
        {
        }

        /// <summary>Runs when the unit deals damage, before the unit's components see it.</summary>
        public virtual void OnDealtDamage(Damage damage)
        {
        }

        /// <summary>Runs when the unit dies, before the unit's components see it; the killer is null when no unit killed it. Death is not an end, and the corpse can go at any time (it decays, explodes or is raised), so what this makes belongs to the game: a respawn timer started here runs whatever happens to the corpse. An effect that should end with the corpse names the unit as its owner: Timer.Every(1f, Smoke, this).</summary>
        /// <param name="killerUnit">The unit whose damage killed it; null when no unit did: Kill() or Life set to 0 from code, timed life running out, or its own damage (seen 10-21).</param>
        public virtual void OnDeath(Unit? killerUnit)
        {
        }

        /// <summary>Runs when the unit kills another, before the unit's components see it.</summary>
        /// <param name="killedUnit">The unit it killed.</param>
        public virtual void OnKill(Unit killedUnit)
        {
        }

        /// <summary>Runs when the unit begins a cast, after the cast's ability has seen it and before the unit's components see it.</summary>
        public virtual void OnCast(Cast cast)
        {
        }

        /// <summary>Tells the interactions package, if there is one; runs OnLeave; the components then end, and the unit leaves the game.</summary>
        protected sealed override void OnEnd()
        {
            Hooks.CallEnding(this);
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnLeave();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnLeave", error);
            }

            Owner.current = previous;
        }

        /// <summary>Removes the unit from the game unless the game has already, and forgets it.</summary>
        internal override void Finish()
        {
            if (!IsGone)
            {
                Runtime.World.DeleteUnit(Handle);
                IsGone = true;
            }

            KnownUnits.Forget(this);
        }
    }
}
