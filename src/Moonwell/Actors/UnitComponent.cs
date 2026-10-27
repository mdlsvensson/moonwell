namespace Moonwell
{
    /// <summary>A component on a unit, which reacts to the unit's events.</summary>
    public abstract class UnitComponent : Component
    {
        /// <summary>The table behind ComponentHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? componentHooks;

        /// <summary>The reactions of unit components, named as Unit's are.</summary>
        internal static HookTable ComponentHooks
        {
            get
            {
                if (componentHooks == null)
                {
                    componentHooks = new HookTable(typeof(UnitComponent), new string[] { "OnItemPickedUp", "OnItemDropped", "OnDamaged", "OnDealtDamage", "OnDeath", "OnKill", "OnCast" }, new int[] { Hook.OnItemPickedUp, Hook.OnItemDropped, Hook.OnDamaged, Hook.OnDealtDamage, Hook.OnDeath, Hook.OnKill, Hook.OnCast });
                }

                return componentHooks;
            }
        }

        /// <summary>The unit this component is on.</summary>
        [LocalSafe]
        public Unit Unit => (Unit)Actor;

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => ComponentHooks;

        /// <summary>Runs when the unit picks up an item.</summary>
        /// <param name="pickedItem">The item the unit picked up.</param>
        public virtual void OnItemPickedUp(Item pickedItem)
        {
        }

        /// <summary>Runs when the unit drops an item.</summary>
        /// <param name="droppedItem">The item the unit dropped.</param>
        public virtual void OnItemDropped(Item droppedItem)
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

        /// <summary>Runs when the unit dies; the killer is null when no unit killed it. What this makes belongs to the component, which ends when the unit leaves the game, as its corpse goes; a respawn timer names the game as its owner: Timer.After(60f, Respawn, Game.Instance).</summary>
        /// <param name="killerUnit">The unit whose damage killed it; null when no unit did: Kill() or Life set to 0 from code, timed life running out, or its own damage (seen 10-21).</param>
        public virtual void OnDeath(Unit? killerUnit)
        {
        }

        /// <summary>Runs when the unit kills another.</summary>
        /// <param name="killedUnit">The unit it killed.</param>
        public virtual void OnKill(Unit killedUnit)
        {
        }

        /// <summary>Runs when the unit begins a cast.</summary>
        public virtual void OnCast(Cast cast)
        {
        }
    }
}
