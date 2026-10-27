namespace Moonwell
{
    /// <summary>A component on a destructable, which reacts to the destructable's events.</summary>
    public abstract class DestructableComponent : Component
    {
        /// <summary>The table behind ComponentHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? componentHooks;

        /// <summary>The reactions of destructable components, named as Destructable's are.</summary>
        internal static HookTable ComponentHooks
        {
            get
            {
                if (componentHooks == null)
                {
                    componentHooks = new HookTable(typeof(DestructableComponent), new string[] { "OnDeath" }, new int[] { Hook.OnDeath });
                }

                return componentHooks;
            }
        }

        /// <summary>The destructable this component is on; null before the component is added, though its type does not say so (read it from OnAdded on).</summary>
        [LocalSafe]
        public Destructable Destructable => (Destructable)Actor;

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => ComponentHooks;

        /// <summary>Runs when the destructable dies.</summary>
        public virtual void OnDeath()
        {
        }
    }
}
