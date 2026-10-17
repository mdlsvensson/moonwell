namespace Moonwell
{
    /// <summary>A component on a unit, which reacts to the unit's events.</summary>
    public abstract class UnitComponent : Component
    {
        /// <summary>The reactions of unit components, named as Unit's are.</summary>
        internal static readonly HookTable ComponentHooks = new HookTable(typeof(UnitComponent), new string[] { "OnItemPickedUp", "OnItemDropped" }, new int[] { Hook.OnItemPickedUp, Hook.OnItemDropped });

        /// <summary>The unit this component is on.</summary>
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
    }
}
