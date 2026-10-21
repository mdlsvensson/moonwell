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
