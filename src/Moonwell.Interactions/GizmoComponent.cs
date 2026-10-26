namespace Moonwell
{
    /// <summary>A component on a gizmo. It reacts to no game event: the package that builds on gizmos calls the reactions of its own kind of component itself.</summary>
    public abstract class GizmoComponent : Component
    {
        /// <summary>The table behind ComponentHooks, made on first use so that the class has no static initializer (see UnitComponent).</summary>
        private static HookTable? componentHooks;

        /// <summary>The reactions of gizmo components: none.</summary>
        internal static HookTable ComponentHooks
        {
            get
            {
                if (componentHooks == null)
                {
                    componentHooks = new HookTable(typeof(GizmoComponent), new string[0], new int[0]);
                }

                return componentHooks;
            }
        }

        /// <summary>The gizmo this component is on.</summary>
        [LocalSafe]
        public Gizmo Gizmo => (Gizmo)Actor;

        /// <summary>The reactions this kind of component declares: none.</summary>
        internal override HookTable Table => ComponentHooks;
    }
}
