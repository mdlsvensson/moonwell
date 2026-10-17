namespace Moonwell
{
    /// <summary>A component on a unit, which reacts to the unit's events.</summary>
    public abstract class UnitComponent : Component
    {
        /// <summary>The reactions of unit components, named as Unit's are; empty until the reactions are declared.</summary>
        internal static readonly HookTable ComponentHooks = new HookTable(typeof(UnitComponent), new string[] { }, new int[] { });

        /// <summary>The unit this component is on.</summary>
        public Unit Unit => (Unit)Actor;

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => ComponentHooks;
    }
}
