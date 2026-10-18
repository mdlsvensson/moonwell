namespace Moonwell
{
    /// <summary>A component on a unit, which reacts to the unit's events.</summary>
    public abstract class UnitComponent : Component
    {
        /// <summary>The reactions of unit components, named as Unit's are.</summary>
        internal static readonly HookTable ComponentHooks = new HookTable(typeof(UnitComponent), new string[] { "OnItemPickedUp", "OnItemDropped", "OnDamaged", "OnDealtDamage", "OnDeath", "OnKill", "OnCast" }, new int[] { Hook.OnItemPickedUp, Hook.OnItemDropped, Hook.OnDamaged, Hook.OnDealtDamage, Hook.OnDeath, Hook.OnKill, Hook.OnCast });

        /// <summary>The unit this component is on.</summary>
        [LocalSafe]
        public Unit Unit => (Unit)Actor;

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => ComponentHooks;

        /// <summary>Runs when the unit picks up an item.</summary>
        public virtual void OnItemPickedUp(Item item)
        {
        }

        /// <summary>Runs when the unit drops an item.</summary>
        public virtual void OnItemDropped(Item item)
        {
        }

        /// <summary>Runs when the unit takes damage.</summary>
        public virtual void OnDamaged(Damage damage)
        {
        }

        /// <summary>Runs when the unit deals damage.</summary>
        public virtual void OnDealtDamage(Damage damage)
        {
        }

        /// <summary>Runs when the unit dies; the killer is null when none is known.</summary>
        public virtual void OnDeath(Unit? killer)
        {
        }

        /// <summary>Runs when the unit kills another.</summary>
        public virtual void OnKill(Unit victim)
        {
        }

        /// <summary>Runs when the unit begins a cast.</summary>
        public virtual void OnCast(Cast cast)
        {
        }
    }
}
