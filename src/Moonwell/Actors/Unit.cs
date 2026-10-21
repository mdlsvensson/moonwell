using System;
using System.Collections.Generic;

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

        /// <summary>The ability objects of the unit, one per ability id, in the order they were first asked for.</summary>
        private readonly OrderedMap<int, Ability> abilities = new OrderedMap<int, Ability>();

        /// <summary>The game's handle of the unit.</summary>
        [LocalSafe]
        public unit Handle { get; internal set; } = null!;

        /// <summary>The type id of the unit, as the game read it when the unit was taken in.</summary>
        [LocalSafe]
        public int TypeId { get; internal set; }

        /// <summary>The game's player that owned the unit when it was last read; kept for when the game no longer gives it.</summary>
        internal player owner = null!;

        /// <summary>The player that owns the unit; once the game has removed the unit, the last owner read.</summary>
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
                        owner = now;
                    }
                }

                return Player.Of(owner)!;
            }
        }

        /// <summary>The life last read, kept for when the game no longer gives it.</summary>
        private float lastLife;

        /// <summary>The most life last read.</summary>
        private float lastMaxLife;

        /// <summary>The mana last read.</summary>
        private float lastMana;

        /// <summary>The most mana last read.</summary>
        private float lastMaxMana;

        /// <summary>The x last read.</summary>
        private float lastX;

        /// <summary>The y last read.</summary>
        private float lastY;

        /// <summary>The facing last read.</summary>
        private float lastFacing;

        /// <summary>The level last read.</summary>
        private int lastLevel;

        /// <summary>Whether the unit is paused, as last read or set.</summary>
        private bool lastPaused;

        /// <summary>The scale last read or set.</summary>
        private float lastScale = 1.0f;

        /// <summary>Whether the scale has been read or set, so that lastScale holds it.</summary>
        private bool scaleKnown;

        /// <summary>The fly height last read or set.</summary>
        private float lastFlyHeight;

        /// <summary>The inventory size last read.</summary>
        private int lastInventorySize;

        /// <summary>The unit's life; setting it to less than 0.405 kills the unit. Once the game has removed the unit, the last life read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Life
        {
            get
            {
                if (!IsGone)
                {
                    lastLife = Runtime.World.LifeOf(Handle);
                }

                return lastLife;
            }

            set
            {
                if (!IsGone)
                {
                    lastLife = value;
                    Runtime.World.SetLife(Handle, value);
                }
            }
        }

        /// <summary>The most life the unit can have. Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float MaxLife
        {
            get
            {
                if (!IsGone)
                {
                    lastMaxLife = Runtime.World.MaxLifeOf(Handle);
                }

                return lastMaxLife;
            }

            set
            {
                if (!IsGone)
                {
                    lastMaxLife = value;
                    Runtime.World.SetMaxLife(Handle, value);
                }
            }
        }

        /// <summary>The unit's mana. Once the game has removed the unit, the last mana read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Mana
        {
            get
            {
                if (!IsGone)
                {
                    lastMana = Runtime.World.ManaOf(Handle);
                }

                return lastMana;
            }

            set
            {
                if (!IsGone)
                {
                    lastMana = value;
                    Runtime.World.SetMana(Handle, value);
                }
            }
        }

        /// <summary>The most mana the unit can have. Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float MaxMana
        {
            get
            {
                if (!IsGone)
                {
                    lastMaxMana = Runtime.World.MaxManaOf(Handle);
                }

                return lastMaxMana;
            }

            set
            {
                if (!IsGone)
                {
                    lastMaxMana = value;
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
                if (!IsGone)
                {
                    lastX = Runtime.World.UnitX(Handle);
                }

                return lastX;
            }
        }

        /// <summary>The y of the unit; once the game has removed it, the last y read.</summary>
        [LocalSafe]
        public float Y
        {
            get
            {
                if (!IsGone)
                {
                    lastY = Runtime.World.UnitY(Handle);
                }

                return lastY;
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

        /// <summary>Puts the unit at a place with no pathing (SetUnitX and SetUnitY): it may stand where it could not walk, and its order goes on. Does nothing once the unit is gone.</summary>
        public void PlaceAt(float x, float y)
        {
            if (!IsGone)
            {
                Runtime.World.PlaceUnit(Handle, x, y);
            }
        }

        /// <summary>The unit's facing in degrees. Setting it makes the unit turn at its turn rate (SetUnitFacing), so a read just after still gives the old facing; FaceNow turns at once. Once the game has removed the unit, the last facing read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Facing
        {
            get
            {
                if (!IsGone)
                {
                    lastFacing = Runtime.World.FacingOf(Handle);
                }

                return lastFacing;
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

        /// <summary>Turns the unit to a facing in degrees at once (BlzSetUnitFacingEx). Does nothing once the unit is gone.</summary>
        public void FaceNow(float degrees)
        {
            if (!IsGone)
            {
                Runtime.World.FaceUnitNow(Handle, degrees);
            }
        }

        /// <summary>The unit's level; a hero's hero level. Once the game has removed the unit, the last level read.</summary>
        [LocalSafe]
        public int Level
        {
            get
            {
                if (!IsGone)
                {
                    lastLevel = Runtime.World.LevelOf(Handle);
                }

                return lastLevel;
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
            }
        }

        /// <summary>Whether the unit is paused (PauseUnit). Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public bool IsPaused
        {
            get
            {
                if (!IsGone)
                {
                    lastPaused = Runtime.World.IsPausedUnit(Handle);
                }

                return lastPaused;
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

        /// <summary>The size of the unit's model as a factor of its normal size, set on all three axes (SetUnitScale). Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float Scale
        {
            get
            {
                if (!IsGone && !scaleKnown)
                {
                    lastScale = Runtime.World.ScaleOf(Handle);
                    scaleKnown = true;
                }

                return lastScale;
            }

            set
            {
                if (!IsGone)
                {
                    lastScale = value;
                    scaleKnown = true;
                    Runtime.World.ScaleUnit(Handle, value);
                }
            }
        }

        /// <summary>Tints the unit's model (SetUnitVertexColor); each value is 0-255 and an alpha of 0 makes the unit invisible. The game keeps no tint to read back. Does nothing once the unit is gone.</summary>
        public void Tint(int red, int green, int blue, int alpha = 255)
        {
            if (!IsGone)
            {
                Runtime.World.TintUnit(Handle, red, green, blue, alpha);
            }
        }

        /// <summary>How high the unit flies above the ground, set at once (SetUnitFlyHeight with a rate of 0). Once the game has removed the unit, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public float FlyHeight
        {
            get
            {
                if (!IsGone)
                {
                    lastFlyHeight = Runtime.World.FlyHeightOf(Handle);
                }

                return lastFlyHeight;
            }

            set
            {
                if (!IsGone)
                {
                    lastFlyHeight = value;
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
                if (!IsGone)
                {
                    lastInventorySize = Runtime.World.InventorySizeOf(Handle);
                }

                return lastInventorySize;
            }
        }

        /// <summary>The item in a slot of the unit's inventory, counting from 0; null when the slot is empty, when there is no such slot, and once the unit is gone. Not for Local.For code: an item Moonwell has not seen is taken in (its components and OnEnter run), which must happen on every machine.</summary>
        public Item? ItemIn(int slot)
        {
            if (IsGone || slot < 0 || slot >= InventorySize)
            {
                return null;
            }

            return Item.Of(Runtime.World.ItemInSlot(Handle, slot));
        }

        /// <summary>Gives the unit an item to pick up into its first free slot, and returns the game's answer (false when the inventory is full). The game counts it as a pick-up, so OnPickedUp and OnItemPickedUp run, inside this call. False, and no call to the game, once the unit or the item is gone.</summary>
        public bool Give(Item item)
        {
            return !IsGone && !item.IsGone && Runtime.World.GiveItem(Handle, item.Handle);
        }

        /// <summary>Makes the unit drop an item from its inventory where it stands. The game counts it as a drop, so OnDropped and OnItemDropped run, inside this call. Does nothing once the unit or the item is gone.</summary>
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

            return Units.TakeIn(handle, null);
        }

        /// <summary>The facing of a unit made without one, in degrees: the game's bj_UNIT_FACING.</summary>
        public const float DefaultFacing = 270.0f;

        /// <summary>Creates a unit of a type for a player at a place and facing (270 degrees without one); owner, when given, owns it.</summary>
        public static Unit Create(int typeId, Player player, float x, float y, float facing = DefaultFacing, Owner? owner = null)
        {
            var handle = Runtime.World.SpawnUnit(player.Handle, typeId, x, y, facing);
            var made = Units.TakeIn(handle, owner)!;
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

        /// <summary>Applies a buff to the unit and returns the buff now on it: a buff of its class already there is refreshed, as its stacking says, and the given buff is dropped; otherwise the given buff is added and started.</summary>
        public T Apply<T>(T buff, Unit? source = null)
            where T : Buff
        {
            Dev.Check(buff.Duration >= 0.0f && buff.Tick >= 0.0f, "Unit.Apply: Duration and Tick must be 0 or more");
            var existing = StackedWith(buff, source);
            if (existing != null)
            {
                existing.Source = source;
                existing.Reapply();
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

        /// <summary>Removes the unit from the game: it ends, which runs its OnLeave.</summary>
        public void Remove() => End();

        /// <summary>Runs when the unit enters the game, after its starting components are added.</summary>
        protected virtual void OnEnter()
        {
        }

        /// <summary>Runs when the unit leaves the game, before its components end.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Runs OnEnter; Units calls it, since a map overrides the hook as protected.</summary>
        internal void CallEnter()
        {
            OnEnter();
        }

        /// <summary>Runs when the unit picks up an item, before the unit's components see it.</summary>
        public virtual void OnItemPickedUp(Item item)
        {
        }

        /// <summary>Runs when the unit drops an item, before the unit's components see it.</summary>
        public virtual void OnItemDropped(Item item)
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

        /// <summary>Runs when the unit dies, before the unit's components see it; the killer is null when none is known.</summary>
        public virtual void OnDeath(Unit? killer)
        {
        }

        /// <summary>Runs when the unit kills another, before the unit's components see it.</summary>
        public virtual void OnKill(Unit victim)
        {
        }

        /// <summary>Runs when the unit begins a cast, after the cast's ability has seen it and before the unit's components see it.</summary>
        public virtual void OnCast(Cast cast)
        {
        }

        /// <summary>Runs OnLeave first; the components then end, and the unit leaves the game.</summary>
        protected sealed override void OnEnd()
        {
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

            Units.Forget(this);
        }
    }
}
