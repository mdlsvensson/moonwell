namespace Moonwell
{
    /// <summary>A component on a destructable, which reacts to the destructable's events.</summary>
    public abstract class DestructableComponent : Component
    {
        /// <summary>The reactions of destructable components, named as Destructable's are.</summary>
        internal static readonly HookTable ComponentHooks = new HookTable(typeof(DestructableComponent), new string[] { "OnDeath" }, new int[] { Hook.OnDeath });

        /// <summary>The destructable this component is on.</summary>
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
